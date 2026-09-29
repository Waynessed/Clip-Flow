import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {randomUUID} from 'node:crypto';
import path from 'node:path';
const exec=promisify(execFile),root=path.resolve(import.meta.dirname,'..');
if(process.platform==='win32')process.env.DOCKER_CONTEXT='desktop-linux';
const compose=async(...args)=>(await exec('docker',['compose',...args],{cwd:root,maxBuffer:2*1024*1024})).stdout;
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const data=await readFile(path.join(root,'.artifacts/demo.mp4'));
const api=async(url,options)=>{const r=await fetch(`http://localhost:8080${url}`,options);const value=await r.json();if(!r.ok)throw new Error(JSON.stringify(value));return value;};
async function submit(){const form=new FormData();form.append('file',new Blob([data],{type:'video/mp4'}),'reliability.mp4');return api('/v1/jobs',{method:'POST',headers:{'Idempotency-Key':randomUUID()},body:form});}
async function wait(id,state){const deadline=Date.now()+180_000;while(Date.now()<deadline){const j=await api(`/v1/jobs/${id}`);if(j.state===state)return j;if(j.state==='failed'&&state!=='failed')throw new Error(j.error_message);await sleep(500)}throw new Error(`Timed out waiting for ${state}`);}
async function ready(){const deadline=Date.now()+60_000;while(Date.now()<deadline){try{await api('/readyz');return}catch{}await sleep(500)}throw new Error('Readiness did not recover');}
const evidence={started_at:new Date().toISOString()};
try{
 await compose('stop','worker');const queued=await submit();if(queued.state!=='queued')throw new Error('Expected durable queued job');
 await compose('restart','api');await ready();const survived=await api(`/v1/jobs/${queued.id}`);if(survived.state!=='queued')throw new Error('API restart lost queued work');
 await compose('up','-d','--no-deps','worker');evidence.restart=await wait(queued.id,'succeeded');console.log('Queued work survived API/worker restart and completed.');
 await compose('stop','worker');const interrupted=await submit();await compose('stop','minio');await compose('up','-d','--no-deps','worker');
 const exhausted=await wait(interrupted.id,'failed');if(exhausted.attempt_count!==3||exhausted.error_category!=='storage_unavailable')throw new Error('Storage interruption did not exhaust exactly three attempts');
 evidence.storage_interruption=exhausted;console.log('Real storage outage caused bounded retries and failed after three attempts.');
}catch(e){evidence.error=e.message;throw e}finally{
 await compose('up','-d','--no-deps','minio');await ready();await compose('up','-d','--no-deps','worker');
 evidence.ended_at=new Date().toISOString();await mkdir(path.join(root,'.artifacts'),{recursive:true});await writeFile(path.join(root,'.artifacts/reliability.json'),JSON.stringify(evidence,null,2));
}
