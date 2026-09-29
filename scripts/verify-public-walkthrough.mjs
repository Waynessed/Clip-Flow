import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { chromium } from '../web/node_modules/@playwright/test/index.mjs';

const base = 'https://waynessed.github.io/Clip-Flow/';
const root = fileURLToPath(new URL('../', import.meta.url));
const integrity = JSON.parse(await readFile(`${root}/web/walkthrough-public/integrity.json`, 'utf8'));
const observed = await Promise.all(Object.entries(integrity.assets).map(async ([file, expected]) => {
  const response = await fetch(new URL(file, base), { signal: AbortSignal.timeout(30000) });
  assert.equal(response.status, 200, `${file}: public request failed`);
  assert.equal(new URL(response.url).protocol, 'https:');
  const bytes = Buffer.from(await response.arrayBuffer());
  const hash = createHash('sha256').update(bytes).digest('hex');
  assert.equal(hash, expected.sha256, `${file}: deployed content differs from verified export`);
  return { file, status: response.status, bytes: bytes.length, sha256: hash };
}));
const browser = await chromium.launch();
const requests = [], errors = [];
let playback;
try {
  const page = await browser.newPage({ viewport: { width: 1360, height: 1000 } });
  page.on('request', request => requests.push({ method: request.method(), url: request.url() }));
  page.on('pageerror', error => errors.push(error.message));
  const response = await page.goto(base, { waitUntil: 'networkidle', timeout: 30000 });
  assert.equal(response.status(), 200);
  assert.equal(await page.locator('form, input[type=file]').count(), 0);
  assert.match(await page.locator('meta[http-equiv="Content-Security-Policy"]').getAttribute('content'), /form-action 'none'/);
  await page.waitForFunction(() => document.querySelector('video')?.videoHeight === 480);
  playback = await page.locator('video').evaluate(async video => {
    video.muted = true;
    await video.play();
    await new Promise(resolve => setTimeout(resolve, 700));
    video.pause();
    return { width: video.videoWidth, height: video.videoHeight, duration: video.duration, currentTime: video.currentTime };
  });
  assert.ok(playback.currentTime > 0.2);
  await page.getByRole('button', { name: /An obsolete result was rejected/ }).click();
  assert.match(await page.locator('.rejection').innerText(), /stale_publication/);
  await page.getByRole('button', { name: /A bounded storage failure/ }).click();
  assert.equal(await page.locator('.attempts li').count(), 3);
  await page.setViewportSize({ width: 390, height: 844 });
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  assert.deepEqual(errors, []);
  assert.ok(requests.every(request => request.method === 'GET' && request.url.startsWith(base)));
} finally {
  await browser.close();
}
const evidence = { checked_at: new Date().toISOString(), url: base, success: true, playback,
  public_asset_checks: observed, browser_requests: requests, browser_errors: errors,
  notes: 'Actual HTTPS asset/hash and browser checks; no local backend or uploads involved.' };
await writeFile(`${root}/docs/evidence/public-walkthrough-live.json`, JSON.stringify(evidence, null, 2) + '\n');
console.log(JSON.stringify({ url: base, success: true, verified_assets: observed.length, playback }));
