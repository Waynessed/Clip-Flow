import {readFile,writeFile,mkdir,readdir} from 'node:fs/promises';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {randomUUID,createHash} from 'node:crypto';
import {performance} from 'node:perf_hooks';
import path from 'node:path';
const exec=promisify(execFile),root=path.resolve(import.meta.dirname,'..');
process.env.DOCKER_CONTEXT='desktop-linux';
const docker=async(...args)=>(await exec('docker',args,{cwd:root,maxBuffer:10*1024*1024})).stdout.trim();
const compose=(...args)=>docker('compose',...args);
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const artifact=path.join(root,'docs','benchmarks',new Date().toISOString().replaceAll(':','-'));
await mkdir(artifact,{recursive:true});
async function api(url,options){const r=await fetch(`http://localhost:8080${url}`,options);const value=await r.json();if(!r.ok)throw new Error(JSON.stringify(value));return value;}
const clipsDir=path.join(root,'.artifacts','benchmark-clips');
const files=(await readdir(clipsDir)).filter(s=>s.endsWith('.mp4')).sort();
if(files.length!==100)throw new Error(`Expected 100 synthetic clips; got ${files.length}. Run scripts/benchmark.ps1.`);
const clips=await Promise.all(files.map(async name=>({name,data:await readFile(path.join(clipsDir,name))})));
const fixtureManifest=clips.map(c=>({name:c.name,bytes:c.data.length,sha256:createHash('sha256').update(c.data).digest('hex')}));
await writeFile(path.join(artifact,'fixtures.json'),JSON.stringify(fixtureManifest,null,2));
const hardware={recorded_at:new Date().toISOString(),docker_info:await docker('info','--format','{{json .}}'),host:await exec('powershell',['-NoProfile','-Command','Get-CimInstance Win32_Processor | Select-Object Name,NumberOfCores,NumberOfLogicalProcessors | ConvertTo-Json'],{cwd:root}).then(r=>r.stdout).catch(e=>e.message),worker_limits:{cpus:1,memory_bytes:536870912},fixture:'100 distinct 1-second 960x540/12fps synthetic clips, no audio',revision:await exec('git',['-c','safe.directory=D:/Projects/ClipFlow','rev-parse','HEAD'],{cwd:root}).then(r=>r.stdout.trim()),compose:await compose('config')};
await writeFile(path.join(artifact,'environment.json'),JSON.stringify(hardware,null,2));
const memoryMiB=s=>{const match=s.split('/')[0].trim().match(/^([\d.]+)\s*(B|KiB|MiB|GiB)$/);return match?Number(match[1])*({B:1/1048576,KiB:1/1024,MiB:1,GiB:1024}[match[2]]):0};
const results=[];
try{
 for(const workers of [1,2,4])for(let repetition=1;repetition<=3;repetition++){
  const raw={workers,repetition,started_at:new Date().toISOString(),jobs:[],samples:[],status:'running'};
  const rawPath=path.join(artifact,`workers-${workers}-run-${repetition}.json`);
  let sampler;
  try{
   await compose('stop','worker');
   const pending=await api('/v1/jobs');if(pending.some(j=>['queued','running'].includes(j.state)))throw new Error('Queue contains unrelated pending jobs. Finish those before benchmarking.');
   console.log(`Benchmark: ${workers} workers, repetition ${repetition}; submitting 100 clips`);
   let next=0;
   await Promise.all(Array.from({length:4},async()=>{while(next<clips.length){const c=clips[next++],start=new Date().toISOString(),form=new FormData();form.append('file',new Blob([c.data],{type:'video/mp4'}),c.name);const j=await api('/v1/jobs',{method:'POST',headers:{'Idempotency-Key':randomUUID()},body:form});raw.jobs.push({id:j.id,request_started_at:start,filename:c.name})}}));
   raw.worker_start_requested_at=new Date().toISOString();const begin=performance.now();
   await compose('up','-d','--no-deps','--scale',`worker=${workers}`,'worker');
   const ids=(await compose('ps','-q','worker')).split(/\r?\n/).filter(Boolean);
   let sampling=false;
   const sample=async()=>{if(sampling)return;sampling=true;try{const out=await docker('stats','--no-stream','--format','{{json .}}',...ids);raw.samples.push({at:new Date().toISOString(),containers:out.split(/\r?\n/).map(s=>JSON.parse(s))})}catch(e){raw.samples.push({at:new Date().toISOString(),error:e.message})}finally{sampling=false}};
   sampler=setInterval(sample,2000);await sample();
   const waiting=new Map(raw.jobs.map(j=>[j.id,j]));const deadline=Date.now()+15*60*1000;
   while(waiting.size&&Date.now()<deadline){
    const batch=[...waiting.values()];let index=0;
    await Promise.all(Array.from({length:8},async()=>{while(index<batch.length){const entry=batch[index++],detail=await api(`/v1/jobs/${entry.id}`);if(['succeeded','failed'].includes(detail.state)){entry.detail=detail;waiting.delete(entry.id)}}}));
    console.log(`  ${100-waiting.size}/100 terminal`);if(waiting.size)await sleep(1000);
   }
   raw.elapsed_seconds=(performance.now()-begin)/1000;
   if(waiting.size)throw new Error(`${waiting.size} jobs did not finish within 15 minutes`);
   const failed=raw.jobs.filter(j=>j.detail.state!=='succeeded');if(failed.length)throw new Error(`${failed.length} jobs failed`);
   raw.status='succeeded';
  }catch(e){raw.status='failed';raw.error=e.message;console.error(`Run failed: ${e.message}`)}finally{if(sampler)clearInterval(sampler);raw.ended_at=new Date().toISOString();await writeFile(rawPath,JSON.stringify(raw,null,2));results.push(raw)}
 }
}finally{await compose('up','-d','--no-deps','--scale','worker=1','worker')}
const percentile=(xs,p)=>{const a=[...xs].sort((a,b)=>a-b);return a.length?a[Math.ceil(p*a.length)-1]:null};
const summary=results.map(r=>{
 const jobs=r.jobs.filter(j=>j.detail?.state==='succeeded');const waits=jobs.map(j=>(Date.parse(j.detail.attempts[0].started_at)-Date.parse(j.detail.created_at))/1000),e2e=jobs.map(j=>(Date.parse(j.detail.completed_at)-Date.parse(j.detail.created_at))/1000);
 const original=jobs.reduce((n,j)=>n+j.detail.metadata.input.size,0),preview=jobs.reduce((n,j)=>n+j.detail.metadata.preview.size,0);
 const memorySamples=r.samples.filter(s=>s.containers).map(s=>s.containers.reduce((n,c)=>n+memoryMiB(c.MemUsage),0));
 const cpuSamples=r.samples.filter(s=>s.containers).map(s=>s.containers.reduce((n,c)=>n+parseFloat(c.CPUPerc),0));
 return {workers:r.workers,repetition:r.repetition,status:r.status,error:r.error,completed:jobs.length,elapsed_seconds:r.elapsed_seconds,jobs_per_minute:r.elapsed_seconds?jobs.length/r.elapsed_seconds*60:null,median_queue_wait_seconds:percentile(waits,.5),p95_queue_wait_seconds:percentile(waits,.95),median_e2e_seconds:percentile(e2e,.5),p95_e2e_seconds:percentile(e2e,.95),preview_reduction_percent:original?(1-preview/original)*100:null,mean_total_worker_cpu_percent:cpuSamples.length?cpuSamples.reduce((a,b)=>a+b,0)/cpuSamples.length:null,peak_total_worker_cpu_percent:cpuSamples.length?Math.max(...cpuSamples):null,peak_total_worker_memory_mib:memorySamples.length?Math.max(...memorySamples):null};
});
await writeFile(path.join(artifact,'summary.json'),JSON.stringify(summary,null,2));
const f=n=>n==null?'n/a':n.toFixed(2);
const report=`# Measured local benchmark\n\nRecorded ${new Date().toISOString()}. Revision ${hardware.revision} plus working benchmark scripts.\n\n100 distinct clips per run, 1/2/4 workers, three repetitions each. Workers limited to one CPU and 512 MiB each. Docker engine allocation: 8 logical CPUs / 7.648 GiB. See environment.json for host CPU, exact Compose and engine data; fixtures.json for input hashes.\n\n| Workers | Run | Status | Completed | Jobs/min | Queue median / p95 (s) | E2E median / p95 (s) | Preview reduction | Mean / peak CPU (%) | Peak total memory (MiB) |\n|---|---|---|---|---|---|---|---|---|---|\n${summary.map(s=>`| ${s.workers} | ${s.repetition} | ${s.status} | ${s.completed} | ${f(s.jobs_per_minute)} | ${f(s.median_queue_wait_seconds)} / ${f(s.p95_queue_wait_seconds)} | ${f(s.median_e2e_seconds)} / ${f(s.p95_e2e_seconds)} | ${f(s.preview_reduction_percent)}% | ${f(s.mean_total_worker_cpu_percent)} / ${f(s.peak_total_worker_cpu_percent)} | ${f(s.peak_total_worker_memory_mib)} |`).join('\n')}\n\n## Method and limits\n\nAll 100 uploads are staged with workers stopped and four API requests in flight. Throughput elapsed time starts immediately before Compose starts workers and ends when polling sees all jobs terminal; it includes worker startup and polling overhead. Queue wait is first attempt start minus database creation time, including batch staging. End-to-end is publication minus database creation, so upload validation/storage time before insertion is excluded; raw request timestamps also permit measuring client-observed upload overhead. CPU/memory samples are docker stats snapshots about every two seconds; CPU sums can exceed 100% across workers. Memory strings and per-container samples are in every raw run file.\n\nThis is a warm-cache synthetic workload on one laptop/WSL2 VM, no audio, one-second clips. It does not predict production throughput, longer inputs, cold builds, network storage, or saturated concurrent upload workloads. Database/object storage share host resources. No benchmark speedup is assumed. Failed runs, if any, are retained with reasons.\n`;
await writeFile(path.join(artifact,'report.md'),report);
await writeFile(path.join(root,'docs','benchmark-report.md'),`# Benchmark report\n\n[Latest measured report](benchmarks/${path.basename(artifact)}/report.md). Raw data is stored alongside it.\n`);
console.log(`Benchmark evidence: ${artifact}`);
if(summary.some(s=>s.status!=='succeeded'))process.exitCode=1;
