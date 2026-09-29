import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import data from "./recordings.json";
import "./style.css";

const repository = "https://github.com/Waynessed/Clip-Flow";
const source = `${repository}/blob/${data.implementation_revision}`;
const media = (file: string) => `${import.meta.env.BASE_URL}media/${file}`;
const kb = (size: number) => `${(size / 1024).toFixed(1)} KiB`;
const time = (stamp: string) => new Date(stamp).toISOString().slice(11, 23);
const elapsed = (start: string, end: string) =>
  ((Date.parse(end) - Date.parse(start)) / 1000).toFixed(2);
const stages = [
  {
    name: "Receive",
    title: "Validate before creating work",
    detail:
      "The Go API streams uploads to bounded temporary files, checks real media with ffprobe and hashes the content. A database uniqueness constraint resolves duplicate request keys across concurrent submissions.",
    file: "internal/service/upload.go",
    line: 31,
    tech: "Go · SHA-256 · ffprobe",
  },
  {
    name: "Persist",
    title: "Acknowledge durable jobs",
    detail:
      "The input is written to MinIO before the PostgreSQL job is acknowledged. These are separate operations: an unsuccessful insertion can leave an orphan object for explicit cleanup.",
    file: "internal/queue/queue.go",
    line: 76,
    tech: "PostgreSQL · MinIO · S3 client",
  },
  {
    name: "Claim",
    title: "Give each attempt its own lease",
    detail:
      "Workers claim eligible rows with FOR UPDATE SKIP LOCKED in a short transaction. A unique token and database-clock lease establish ownership; heartbeats renew it while processing runs outside the transaction.",
    file: "internal/queue/queue.go",
    line: 120,
    tech: "Row locks · Lease tokens · Heartbeats",
  },
  {
    name: "Process",
    title: "Produce and inspect real media",
    detail:
      "FFmpeg produces a JPEG and an H.264/AAC preview up to 480 pixels high. Fixed arguments, restricted local protocols and a deadline bound processing. Generated files are inspected again before publication.",
    file: "internal/media/media.go",
    line: 92,
    tech: "FFmpeg · H.264 · AAC",
  },
  {
    name: "Publish",
    title: "Only the valid attempt can publish",
    detail:
      "Outputs use a unique attempt prefix. PostgreSQL publishes the manifest only if the token still owns an unexpired lease. Execution may repeat after a failure; obsolete attempts cannot replace the visible result.",
    file: "internal/queue/queue.go",
    line: 169,
    tech: "Conditional update · Attempt manifest",
  },
];

function Walkthrough() {
  const [selected, setSelected] = useState(0);
  const [view, setView] = useState<"preview" | "input">("preview");
  const [stage, setStage] = useState(0);
  const [mediaError, setMediaError] = useState(false);
  const run = data.recordings[selected];
  const step = stages[stage];
  const metadata = run.metadata;
  return (
    <>
      <a className="skip" href="#explore">
        Skip to walkthrough
      </a>
      <header className="masthead">
        <a
          className="brand"
          href={import.meta.env.BASE_URL}
          aria-label="ClipFlow home"
        >
          <span className="film-mark" aria-hidden="true">
            ▶
          </span>{" "}
          ClipFlow
        </a>
        <nav aria-label="Main navigation">
          <a href="#engineering">How it works</a>
          <a href={repository}>View GitHub repository</a>
        </nav>
      </header>
      <main>
        <section className="intro" aria-labelledby="title">
          <div>
            <h1 id="title">
              A clip. A queue.
              <br />A result you can inspect.
            </h1>
            <p>
              Explore a video processing application built with Go, React,
              PostgreSQL and FFmpeg. Play the generated media and see what
              happened when workers and storage failed.
            </p>
          </div>
          <aside className="scope">
            <span className="scope-dot" aria-hidden="true" />
            <strong>Read-only walkthrough</strong>
            <p>
              These are recorded local runs from 29 September 2026. The files
              and outcomes are real. Exploring them does not start new
              processing.
            </p>
          </aside>
        </section>

        <section
          id="explore"
          className="workspace"
          aria-label="Explore recorded jobs"
        >
          <aside className="scenario-rail">
            <h2>Choose a recorded run</h2>
            <div className="scenario-list">
              {data.recordings.map((record, index) => (
                <button
                  key={record.id}
                  aria-pressed={selected === index}
                  onClick={() => {
                    setSelected(index);
                    setView("preview");
                    setMediaError(false);
                  }}
                >
                  <span className={`status ${record.state}`}>
                    {record.state === "succeeded" ? "Succeeded" : "Failed"}
                  </span>
                  <strong>{record.title}</strong>
                  <span>{record.description}</span>
                  <small>
                    {record.attempts.length}{" "}
                    {record.attempts.length === 1 ? "attempt" : "attempts"} ·{" "}
                    {elapsed(record.created_at, record.completed_at)} s
                  </small>
                </button>
              ))}
            </div>
            <p className="rail-note">
              All clips are synthetic test patterns. No visitor uploads or
              accounts are collected.
            </p>
          </aside>
          <article className="result" aria-labelledby="run-title">
            <div className="result-heading">
              <div>
                <p className="record-date">Recorded on 29 September 2026</p>
                <h2 id="run-title">{run.title}</h2>
              </div>
              <a href={`${source}/docs/evidence/${run.evidence}`}>
                Inspect source evidence
              </a>
            </div>
            {metadata ? (
              <>
                <div className="view-controls" aria-label="Choose media">
                  <button
                    aria-pressed={view === "preview"}
                    onClick={() => {
                      setView("preview");
                      setMediaError(false);
                    }}
                  >
                    Processed preview
                  </button>
                  <button
                    aria-pressed={view === "input"}
                    onClick={() => {
                      setView("input");
                      setMediaError(false);
                    }}
                  >
                    Original input
                  </button>
                  <span>
                    {view === "input"
                      ? "960 × 540"
                      : `${metadata.preview.width} × ${metadata.preview.height}`}
                  </span>
                </div>
                <div className="video-frame">
                  <video
                    key={`${run.slug}-${view}`}
                    controls
                    playsInline
                    preload="metadata"
                    poster={
                      view === "preview"
                        ? media(`${run.slug}/thumbnail.jpg`)
                        : undefined
                    }
                    aria-label={
                      view === "preview"
                        ? "Processed preview"
                        : "Original input"
                    }
                    onError={() => setMediaError(true)}
                  >
                    <source
                      src={
                        view === "input"
                          ? media("input.mp4")
                          : media(`${run.slug}/preview.mp4`)
                      }
                      type="video/mp4"
                    />
                    Your browser does not support MP4 playback.
                  </video>
                </div>
                {mediaError && (
                  <p role="alert">
                    The clip could not load. Try the direct preview link below
                    or reload the page.
                  </p>
                )}
                <div className="outputs">
                  <a href={media(`${run.slug}/preview.mp4`)}>Open preview</a>
                  <a href={media(`${run.slug}/thumbnail.jpg`)}>
                    Open thumbnail
                  </a>
                  <a href={media(`${run.slug}/metadata.json`)}>View metadata</a>
                </div>
                <dl className="media-facts">
                  <div>
                    <dt>Input</dt>
                    <dd>
                      {kb(metadata.input.size)}
                      <small>
                        {metadata.input.duration.toFixed(3)} s ·{" "}
                        {metadata.input.width} × {metadata.input.height}
                      </small>
                    </dd>
                  </div>
                  <div>
                    <dt>Preview</dt>
                    <dd>
                      {kb(metadata.preview.size)}
                      <small>
                        {metadata.preview.duration.toFixed(3)} s ·{" "}
                        {metadata.preview.width} × {metadata.preview.height}
                      </small>
                    </dd>
                  </div>
                  <div>
                    <dt>Thumbnail</dt>
                    <dd>
                      {kb(metadata.sizes.thumbnail)}
                      <small>JPEG, first usable frame</small>
                    </dd>
                  </div>
                </dl>
              </>
            ) : (
              <div className="failure-panel">
                <span aria-hidden="true">!</span>
                <h3>Storage could not be reached</h3>
                <p>
                  The job retried after five seconds, then fifteen seconds.
                  After the third attempt it stopped with{" "}
                  <code>storage_unavailable</code>. No outputs were published.
                </p>
                <p>
                  This recorded failure came from deliberately stopping MinIO in
                  the local reliability demo.
                </p>
              </div>
            )}

            <section className="attempts" aria-labelledby="attempt-title">
              <h3 id="attempt-title">Processing history</h3>
              <p>Recorded timestamps below are UTC.</p>
              <ol>
                {run.attempts.map((attempt) => (
                  <li key={attempt.number}>
                    <div className="attempt-number" aria-hidden="true">
                      {attempt.number}
                    </div>
                    <div>
                      <strong>Attempt {attempt.number}</strong>
                      <span className={`outcome ${attempt.outcome}`}>
                        {attempt.outcome === "queued"
                          ? "Queued for retry"
                          : attempt.outcome}
                      </span>
                      <p>
                        {time(attempt.started_at)} – {time(attempt.ended_at)} ·{" "}
                        {elapsed(attempt.started_at, attempt.ended_at)} s
                      </p>
                      <details>
                        <summary>Worker and error details</summary>
                        <p>
                          Worker: <code>{attempt.worker}</code>
                        </p>
                        <p>
                          Error category:{" "}
                          <code>{attempt.error_category || "none"}</code>
                        </p>
                      </details>
                    </div>
                  </li>
                ))}
              </ol>
              {run.rejection && (
                <div className="rejection">
                  <strong>Obsolete publication rejected</strong>
                  <p>
                    At {time(run.rejection.time)} UTC, the first attempt tried
                    to publish after the replacement succeeded. The log records{" "}
                    <code>{run.rejection.category}</code>; the published
                    manifest stayed unchanged.
                  </p>
                  <a href={`${source}/docs/evidence/final-recovery-stale.log`}>
                    Read the recorded rejection log
                  </a>
                </div>
              )}
            </section>
            <details className="job-details">
              <summary>Recorded job identity</summary>
              <p>
                Job: <code>{run.id}</code>
              </p>
              <p>
                Created: {run.created_at}
                <br />
                Completed: {run.completed_at}
              </p>
            </details>
          </article>
        </section>

        <section
          id="engineering"
          className="engineering"
          aria-labelledby="engineering-title"
        >
          <div className="section-heading">
            <h2 id="engineering-title">Follow the processing path</h2>
            <p>
              Select a stage to inspect the engineering decisions and their
              implementation.
            </p>
          </div>
          <div className="stage-path" aria-label="Processing stages">
            {stages.map((s, i) => (
              <button
                key={s.name}
                aria-pressed={stage === i}
                onClick={() => setStage(i)}
              >
                <span>{i + 1}</span>
                {s.name}
              </button>
            ))}
          </div>
          <article className="stage-detail" aria-live="polite">
            <div>
              <h3>{step.title}</h3>
              <p>{step.detail}</p>
            </div>
            <aside>
              <p>{step.tech}</p>
              <a href={`${source}/${step.file}#L${step.line}`}>
                Read the implementation
              </a>
            </aside>
          </article>
          <p className="architecture-note">
            React browser → Go API → PostgreSQL jobs / MinIO objects. Go workers
            claim jobs, run FFmpeg, and publish through PostgreSQL.
          </p>
        </section>
        <section className="verification" aria-labelledby="verification-title">
          <h2 id="verification-title">Evidence behind the demo</h2>
          <div className="verification-columns">
            <div>
              <h3>Real services, browser checks</h3>
              <p>
                Go integration tests exercise PostgreSQL, MinIO and FFmpeg.
                Playwright uploads a clip and checks advancing video playback.
                The recorded final GitHub Actions run passed from a clean
                checkout.
              </p>
              <a href="https://github.com/Waynessed/Clip-Flow/actions/runs/36531496192">
                View the verified CI run
              </a>
            </div>
            <div>
              <h3>Measured worker comparison</h3>
              <p>
                Nine local benchmark runs processed 900 synthetic jobs
                successfully. Each worker count was repeated three times, using
                100 clips per run.
              </p>
              <details>
                <summary>Inspect benchmark results and limits</summary>
                <div className="table-scroll">
                  <table>
                    <caption>
                      Aggregate local throughput, 29 September 2026
                    </caption>
                    <thead>
                      <tr>
                        <th scope="col">Workers</th>
                        <th scope="col">Jobs/min</th>
                        <th scope="col">Median completion</th>
                      </tr>
                    </thead>
                    <tbody>
                      {data.benchmarks.map((b) => (
                        <tr key={b.workers}>
                          <th scope="row">{b.workers}</th>
                          <td>{b.jobs_per_minute.toFixed(2)}</td>
                          <td>{b.median_e2e_seconds.toFixed(2)} s</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <p>
                  Short synthetic clips on an uncontrolled laptop; each worker
                  had a one-CPU / 512 MiB limit. Completion measures database
                  creation to publication. These results do not establish
                  production scaling.
                </p>
                <a href={`${source}/docs/benchmark-report.md`}>
                  Read measurements and limitations
                </a>
              </details>
            </div>
          </div>
        </section>
      </main>
      <footer>
        <p>ClipFlow · An engineering portfolio project</p>
        <div>
          <a href={`${source}/docs/walkthrough.md`}>
            Implementation walkthrough
          </a>
          <a href={`${source}/docs/implementation-log.md`}>
            Development records
          </a>
        </div>
        <p>
          Public examples are static copies of selected synthetic runs. Full
          upload and recovery demos can be run locally from the repository.
        </p>
      </footer>
    </>
  );
}
createRoot(document.getElementById("root")!).render(<Walkthrough />);
