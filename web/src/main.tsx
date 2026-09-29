import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";

type Attempt = {
  number: number;
  worker: string;
  started_at: string;
  ended_at: string | null;
  outcome: string;
  error_category: string | null;
  error_message: string | null;
};
type Media = { duration: number; width: number; height: number; size: number };
type Job = {
  id: string;
  filename: string;
  state: string;
  attempt_count: number;
  created_at: string;
  error_category: string | null;
  error_message: string | null;
  outputs?: Record<string, string>;
  metadata?: { input: Media; preview: Media; sizes: Record<string, number> };
  attempts: Attempt[];
};
const explain = (category: string | null, message: string | null) =>
  (
    ({
      storage_unavailable:
        "Object storage could not be reached. This attempt could not finish.",
      invalid_media:
        "The stored clip could not be decoded. Choose a valid MP4.",
      timeout:
        "Processing exceeded its time limit. Try a shorter or smaller clip.",
      lease_expired:
        "The worker stopped renewing its lease. Another worker can recover the clip.",
      retry_exhausted: "The maximum of three attempts was reached.",
      processing_failed:
        "The clip could not be converted. Try exporting it as a standard MP4.",
      cancelled:
        "The attempt stopped after losing ownership or being interrupted.",
    }) as Record<string, string>
  )[category || ""] || message;
const bytes = (n: number) =>
  n < 1024 * 1024
    ? `${(n / 1024).toFixed(0)} KB`
    : `${(n / 1024 / 1024).toFixed(2)} MB`;
async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const r = await fetch(path, options);
  const data = await r.json();
  if (!r.ok)
    throw new Error(data.error?.message || `Request failed (${r.status})`);
  return data;
}
function App() {
  const [jobs, setJobs] = useState<Job[]>([]),
    [selected, setSelected] = useState<string | null>(null),
    [detail, setDetail] = useState<Job | null>(null),
    [file, setFile] = useState<File | null>(null),
    [key, setKey] = useState<string>(crypto.randomUUID()),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  useEffect(() => {
    let alive = true;
    const refresh = async () => {
      try {
        const list = await request<Job[]>("/v1/jobs");
        if (alive) {
          setJobs(list);
          if (!selected && list.length) setSelected(list[0].id);
        }
        if (selected) {
          const d = await request<Job>(`/v1/jobs/${selected}`);
          if (alive) setDetail(d);
        }
      } catch (e) {
        if (alive) setError((e as Error).message);
      }
    };
    refresh();
    const timer = setInterval(refresh, 1000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [selected]);
  async function upload(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const form = new FormData();
      form.append("file", file);
      const j = await request<Job>("/v1/jobs", {
        method: "POST",
        headers: { "Idempotency-Key": key },
        body: form,
      });
      setSelected(j.id);
      setDetail(j);
      setNotice(
        `Clip accepted. Job ${j.id.slice(0, 8)}. Reusing this request key returns the same job.`,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="workspace">
      <header>
        <a className="brand" href="/">
          ▰ ClipFlow
        </a>
        <span>Local clip workspace</span>
      </header>
      <main>
        <section className="intro">
          <h1>From clip to preview.</h1>
          <p>
            Upload a short MP4. Follow its processing and inspect the finished
            files.
          </p>
        </section>
        <div className="workbench">
          <aside>
            <form onSubmit={upload}>
              <h2>Add a clip</h2>
              <label className="file-picker">
                {file ? (
                  <>
                    <strong>{file.name}</strong>
                    <span>{bytes(file.size)}</span>
                  </>
                ) : (
                  <>
                    <strong>Choose your MP4</strong>
                    <span>Up to 20 MB and 30 seconds</span>
                  </>
                )}
                <input
                  aria-label="Choose MP4"
                  type="file"
                  accept="video/mp4,.mp4"
                  onChange={(e) => {
                    setFile(e.target.files?.[0] || null);
                    setKey(crypto.randomUUID());
                    setNotice("");
                  }}
                />
              </label>
              <label className="key-label">
                Request key
                <input
                  aria-label="Request key"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                />
              </label>
              <p className="hint">Keep this key to safely repeat an upload.</p>
              <button
                className="new-key"
                type="button"
                disabled={busy}
                onClick={() => {
                  setKey(crypto.randomUUID());
                  setNotice(
                    "New request key ready. Process the clip to create a new job.",
                  );
                }}
              >
                Use a new request key
              </button>
              <button className="primary" disabled={!file || busy}>
                {busy ? "Uploading…" : "Process clip"}
              </button>
            </form>
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
            {notice && (
              <p role="status" className="notice">
                {notice}
              </p>
            )}
            <section className="queue">
              <h2>
                Recent clips <span>{jobs.length}</span>
              </h2>
              {!jobs.length ? (
                <p className="empty">
                  Your clips will appear here after upload.
                </p>
              ) : (
                <ul>
                  {jobs.map((j) => (
                    <li key={j.id}>
                      <button
                        className={selected === j.id ? "job active" : "job"}
                        onClick={() => setSelected(j.id)}
                      >
                        <span className="job-name">{j.filename}</span>
                        <span className={`state ${j.state}`}>{j.state}</span>
                        <small>
                          {new Date(j.created_at).toLocaleTimeString()} /{" "}
                          {j.attempt_count}{" "}
                          {j.attempt_count === 1 ? "attempt" : "attempts"}
                        </small>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </aside>
          <section className="inspection">
            {!detail ? (
              <div className="stage empty-stage">
                <div className="frame-mark">▰</div>
                <h2>Your preview lives here.</h2>
                <p>
                  Add a clip to see its thumbnail, preview and processing
                  history.
                </p>
              </div>
            ) : (
              <>
                <div className="clip-title">
                  <div>
                    <h2>{detail.filename}</h2>
                    <code>{detail.id}</code>
                  </div>
                  <span className={`state ${detail.state}`}>
                    {detail.state}
                  </span>
                </div>
                <div className="stage">
                  {detail.state === "succeeded" && detail.outputs ? (
                    <video
                      key={detail.id}
                      aria-label="Processed preview"
                      controls
                      preload="metadata"
                      poster={detail.outputs.thumbnail}
                      src={detail.outputs.preview}
                    />
                  ) : (
                    <div className="processing">
                      <span className={`process-mark ${detail.state}`}>▰</span>
                      <h3>
                        {detail.state === "queued"
                          ? "Waiting for a worker"
                          : detail.state === "running"
                            ? "Creating your preview"
                            : "Processing stopped"}
                      </h3>
                      <p>
                        {explain(detail.error_category, detail.error_message) ||
                          "This view updates as the job progresses."}
                      </p>
                    </div>
                  )}
                </div>
                {detail.metadata && (
                  <>
                    <div className="facts">
                      <div>
                        <span>Duration</span>
                        <strong>
                          {detail.metadata.input.duration.toFixed(2)} s
                        </strong>
                      </div>
                      <div>
                        <span>Preview</span>
                        <strong>
                          {detail.metadata.preview.width} ×{" "}
                          {detail.metadata.preview.height}
                        </strong>
                      </div>
                      <div>
                        <span>Input / preview</span>
                        <strong>
                          {bytes(detail.metadata.input.size)} /{" "}
                          {bytes(detail.metadata.preview.size)}
                        </strong>
                      </div>
                    </div>
                    <nav className="outputs" aria-label="Generated outputs">
                      <a href={detail.outputs?.thumbnail} target="_blank">
                        Open thumbnail
                      </a>
                      <a href={detail.outputs?.preview} target="_blank">
                        Open preview
                      </a>
                      <a href={detail.outputs?.metadata} target="_blank">
                        View metadata
                      </a>
                    </nav>
                  </>
                )}
                <section className="history">
                  <h2>Processing history</h2>
                  {!detail.attempts.length ? (
                    <p>Waiting for the first attempt.</p>
                  ) : (
                    <ol>
                      {detail.attempts.map((a) => (
                        <li key={a.number}>
                          <div>
                            <strong>Attempt {a.number}</strong>
                            <span className={`state ${a.outcome}`}>
                              {a.outcome}
                            </span>
                          </div>
                          <p>
                            {new Date(a.started_at).toLocaleTimeString()}
                            {a.ended_at
                              ? ` → ${new Date(a.ended_at).toLocaleTimeString()}`
                              : ""}
                          </p>
                          <code>{a.worker}</code>
                          {a.error_message && (
                            <p className="error">
                              {explain(a.error_category, a.error_message)}
                            </p>
                          )}
                        </li>
                      ))}
                    </ol>
                  )}
                </section>
              </>
            )}
          </section>
        </div>
      </main>
      <footer>MP4 input · JPEG thumbnail · H.264 preview up to 480p</footer>
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
