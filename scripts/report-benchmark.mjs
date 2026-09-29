import {readFile,readdir,writeFile} from 'node:fs/promises';
import path from 'node:path';
const root=path.resolve(import.meta.dirname,'..'),base=path.join(root,'docs/benchmarks');
const folder=process.argv[2]||path.join(base,(await readdir(base)).sort().at(-1));
const filenames=(await readdir(folder)).filter(s=>/^workers-\d-run-\d\.json$/.test(s));
const runs=await Promise.all(filenames.map(async s=>JSON.parse(await readFile(path.join(folder,s),'utf8'))));
const percentile=(values,p)=>{const a=[...values].sort((a,b)=>a-b);return a.length?a[Math.ceil(a.length*p)-1]:null};
const memory=s=>{const m=s.split('/')[0].trim().match(/^([\d.]+)\s*(B|KiB|MiB|GiB)$/);return m?Number(m[1])*({B:1/1048576,KiB:1/1024,MiB:1,GiB:1024}[m[2]]):0};
// Derive every reported statistic from persisted raw evidence, not the mutable
// sampler arrays held by the original benchmark process.
const immutableSummary=runs.map(r=>{
 const jobs=r.jobs.filter(j=>j.detail?.state==='succeeded'),samples=r.samples.filter(s=>s.containers);
 const waits=jobs.map(j=>(Date.parse(j.detail.attempts[0].started_at)-Date.parse(j.detail.created_at))/1000),times=jobs.map(j=>(Date.parse(j.detail.completed_at)-Date.parse(j.detail.created_at))/1000);
 const input=jobs.reduce((n,j)=>n+j.detail.metadata.input.size,0),preview=jobs.reduce((n,j)=>n+j.detail.metadata.preview.size,0),cpu=samples.map(s=>s.containers.reduce((n,c)=>n+parseFloat(c.CPUPerc),0)),mem=samples.map(s=>s.containers.reduce((n,c)=>n+memory(c.MemUsage),0));
 return {workers:r.workers,repetition:r.repetition,status:r.status,error:r.error,completed:jobs.length,elapsed_seconds:r.elapsed_seconds,jobs_per_minute:jobs.length/r.elapsed_seconds*60,median_queue_wait_seconds:percentile(waits,.5),p95_queue_wait_seconds:percentile(waits,.95),median_e2e_seconds:percentile(times,.5),p95_e2e_seconds:percentile(times,.95),preview_reduction_percent:(1-preview/input)*100,mean_total_worker_cpu_percent:cpu.reduce((n,c)=>n+c,0)/cpu.length,peak_total_worker_cpu_percent:Math.max(...cpu),peak_total_worker_memory_mib:Math.max(...mem)};
});
await writeFile(path.join(folder,'summary.json'),JSON.stringify(immutableSummary,null,2));
const groups=[1,2,4].map(workers=>{
 const subset=runs.filter(r=>r.workers===workers),jobs=subset.flatMap(r=>r.jobs.filter(j=>j.detail?.state==='succeeded'));
 const waits=jobs.map(j=>(Date.parse(j.detail.attempts[0].started_at)-Date.parse(j.detail.created_at))/1000),times=jobs.map(j=>(Date.parse(j.detail.completed_at)-Date.parse(j.detail.created_at))/1000);
 const input=jobs.reduce((n,j)=>n+j.detail.metadata.input.size,0),preview=jobs.reduce((n,j)=>n+j.detail.metadata.preview.size,0),thumbs=jobs.reduce((n,j)=>n+j.detail.metadata.sizes.thumbnail,0);
 const samples=subset.flatMap(r=>r.samples.filter(s=>s.containers)),cpus=samples.map(s=>s.containers.reduce((n,c)=>n+parseFloat(c.CPUPerc),0)),mem=samples.map(s=>s.containers.reduce((n,c)=>n+memory(c.MemUsage),0));
 return {workers,runs:subset.length,failed_runs:subset.filter(r=>r.status!=='succeeded').length,completed:jobs.length,total_elapsed_seconds:subset.reduce((n,r)=>n+(r.elapsed_seconds||0),0),jobs_per_minute:jobs.length/subset.reduce((n,r)=>n+(r.elapsed_seconds||0),0)*60,median_queue_wait_seconds:percentile(waits,.5),p95_queue_wait_seconds:percentile(waits,.95),median_e2e_seconds:percentile(times,.5),p95_e2e_seconds:percentile(times,.95),preview_reduction_percent:(1-preview/input)*100,combined_media_reduction_percent:(1-(preview+thumbs)/input)*100,mean_total_cpu_percent:cpus.reduce((n,c)=>n+c,0)/cpus.length,peak_total_cpu_percent:Math.max(...cpus),peak_total_memory_mib:Math.max(...mem)};
});
await writeFile(path.join(folder,'comparison.json'),JSON.stringify(groups,null,2));
const f=n=>Number.isFinite(n)?n.toFixed(2):'n/a';
const interpretation=`\n## Comparison across three repetitions\n\nEach worker configuration completed ${groups.map(g=>`${g.completed}/300 (${g.workers} workers)`).join(', ')}. Aggregate throughput below is total successful jobs divided by summed run elapsed time. Median/p95 pool all completed jobs across each configuration's three runs.\n\n| Workers | Jobs/min | Queue median / p95 (s) | E2E median / p95 (s) | Mean / peak CPU (%) | Peak total memory (MiB) | Preview / combined media reduction |\n|---|---|---|---|---|---|---|\n${groups.map(g=>`| ${g.workers} | ${f(g.jobs_per_minute)} | ${f(g.median_queue_wait_seconds)} / ${f(g.p95_queue_wait_seconds)} | ${f(g.median_e2e_seconds)} / ${f(g.p95_e2e_seconds)} | ${f(g.mean_total_cpu_percent)} / ${f(g.peak_total_cpu_percent)} | ${f(g.peak_total_memory_mib)} | ${f(g.preview_reduction_percent)}% / ${f(g.combined_media_reduction_percent)}% |`).join('\n')}\n\nMeasured throughput ratios relative to one worker: two workers ${f(groups[1].jobs_per_minute/groups[0].jobs_per_minute)}×; four workers ${f(groups[2].jobs_per_minute/groups[0].jobs_per_minute)}×. These ratios describe this recorded synthetic batch on one shared laptop, including the scheduling/polling costs stated above. They do not establish a production scaling guarantee. Combined media reduction includes preview + JPEG and excludes the small metadata JSON. Resource values are sampled, so short peaks can be missed.\n\nFailed runs: ${runs.filter(r=>r.status!=='succeeded').length}. All raw runs are retained.\n`;
const old=await readFile(path.join(folder,'report.md'),'utf8');
const columns='| Workers | Run | Status | Completed | Jobs/min | Queue median / p95 (s) | E2E median / p95 (s) | Preview reduction | Mean / peak CPU (%) | Peak total memory (MiB) |';
const runTable=columns+'\n|---|---|---|---|---|---|---|---|---|---|\n'+immutableSummary.map(s=>`| ${s.workers} | ${s.repetition} | ${s.status} | ${s.completed} | ${f(s.jobs_per_minute)} | ${f(s.median_queue_wait_seconds)} / ${f(s.p95_queue_wait_seconds)} | ${f(s.median_e2e_seconds)} / ${f(s.p95_e2e_seconds)} | ${f(s.preview_reduction_percent)}% | ${f(s.mean_total_worker_cpu_percent)} / ${f(s.peak_total_worker_cpu_percent)} | ${f(s.peak_total_worker_memory_mib)} |`).join('\n');
const prefix=old.split(columns)[0],method=old.split('## Method and limits')[1].split('\n## Comparison across three repetitions')[0];
await writeFile(path.join(folder,'report.md'),prefix+runTable+'\n\n## Method and limits'+method+interpretation);
console.log(JSON.stringify(groups,null,2));
