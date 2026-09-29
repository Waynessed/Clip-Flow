# Public walkthrough

## CF-06 scope

The user requested a public, read-only walkthrough on 2026-09-30. This extends
the original local-only handoff. GitHub Pages hosts a separate static build;
the local upload application stays available through Docker Compose.

Visitors can play actual exported synthetic clips, inspect recorded metadata,
and select completed processing/recovery/outage scenarios. Every scenario is
labelled as a recorded run. There is no simulated live progress and no public
API, upload endpoint, database, or worker.

Only the three explicitly selected synthetic successful jobs from the final
demo evidence and the recorded storage-outage job are included. Arbitrary local
jobs, local credentials, and raw infrastructure error messages are excluded.

## Design plan

Use a media review workspace: a scenario rail beside a large video player,
followed by an attempt timeline and an expandable engineering walkthrough.
Keep the slate-blue visual identity: canvas #e8eef4, paper #f7fafd,
ink #203248, blue #285cc7, teal #22634f, muted #586a80. Segoe UI is
the interface face; Georgia gives the introductory headline a quieter voice.
Left-aligned controls and text, with the video as the main visual anchor.

The review against the brief favors this specific clip/attempt layout over
generic portfolio cards or a decorative hero. On mobile the scenario rail
becomes a full-width selector list. Native video controls, semantic headings,
visible keyboard focus and no automatic animations make exploration accessible.

## Deployment and records

The public URL is https://waynessed.github.io/Clip-Flow/. Pages was enabled
through the GitHub API with `build_type=workflow` and `https_enforced=true`.
Pages workflow 36593150641 deployed revision 63da9ef successfully. Actual
HTTPS asset hashes and hosted browser playback were checked afterward; see
docs/evidence/pages-deployment.json and public-walkthrough-live.json.

### Build and verify locally

```powershell
Set-Location D:/Projects/ClipFlow/web
npm ci
npx playwright install chromium
npm run build:walkthrough
npm run test:walkthrough
npm run preview:walkthrough
# Open http://127.0.0.1:4173/Clip-Flow/
```

The preview runs in the foreground; Ctrl+C stops it. The hosted walkthrough
continues serving even when local Docker or the computer is stopped.

From the repository root, with web dependencies and Chromium installed, run
`node scripts/verify-public-walkthrough.mjs` to check the actual published copy.
It checks HTTPS media hashes, browser playback and read-only behavior, then
writes public-walkthrough-live.json only on success.

`scripts/walkthrough-data.mjs` reads the checked-in evidence for four fixed
job IDs, builds public recordings without raw storage errors or internal object
keys, and verifies every exported file against `integrity.json`. Its explicit
`--export` mode needs the preserved local API and exports only the three approved
synthetic successful jobs. It compares their manifests to archived evidence;
the original input SHA-256 was also compared to PostgreSQL's preserved input hash.
Ordinary builds need no Docker, API, database, or secret.

### Publish and maintain

`.github/workflows/pages.yml` runs on relevant pushes to master and can also
be run manually in GitHub Actions. It verifies hashes, builds the separate
static entry, checks browser playback/scenarios/read-only requests, and then
uploads only `web/dist-walkthrough`. Pull requests run checks without deploying.
Pages deployment uses GitHub's short-lived workflow identity, with Pages write
permissions limited to the deployment job. No stored cloud token is required.

The repository's original integration workflow still verifies the local stack.
Public deployments and the local stack's CI are distinct outcomes; a successful
local test or backend CI run alone does not prove a public deployment succeeded.

Only synthetic examples are published, with no visitor accounts, cookies,
analytics, or input forms in the application. GitHub serves the requests; its
platform may record access logs. The build's CSP restricts scripts, styles,
media and connections to the same origin, and disallows forms/objects. Public
API, DB and object-store ports, credentials and worker failpoints are absent
from the deployment artifact. Backend quotas, upload retention and account
authentication will require a separate implementation if public uploads are
ever requested. The existing local cleanup never deletes referenced results.

### Availability, retention and rollback

Use the workflow's deployment status and GitHub Pages settings to check hosting
health. The public site has no health endpoint or live workers. Its media/data
are immutable versioned assets retained in Git; rebuilding does not query or
mutate the local workspace. To update examples, review/export an explicit new
allowlist and commit its data/hashes. Do not export arbitrary visitor clips.
To roll back, revert the walkthrough change on master and let the workflow
publish that version. To take the public site down, unpublish it in the
repository's Pages settings; stopping local Docker does not affect Pages.

HTTPS is enforced at the configured Pages URL. A custom domain and server-side
HTTP headers are not configured. CSP is supplied through an HTML meta element;
header-only directives such as `frame-ancestors` are not claimed. Video byte
range behavior is supplied by the static host, separate from the local API's
full-object streaming limitation.

The deployment follows GitHub's [custom Pages workflow documentation](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).
