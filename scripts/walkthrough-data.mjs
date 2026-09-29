import { readFile, writeFile, mkdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import path from "node:path";
import assert from "node:assert/strict";

const root = fileURLToPath(new URL("../", import.meta.url));
const assets = path.join(root, "web/walkthrough-public");
const readJSON = async (file) =>
  JSON.parse(await readFile(path.join(root, file), "utf8"));
const sha256 = (data) => createHash("sha256").update(data).digest("hex");
const definitions = [
  {
    slug: "processed",
    title: "A processed clip",
    description:
      "A real MP4 became a thumbnail, a smaller preview and metadata.",
    evidence: "final-demo.json",
    id: "fad3a48b-ee29-4c19-ba78-0f406dc7f2d8",
  },
  {
    slug: "recovered",
    title: "Recovery after worker termination",
    description:
      "The first worker was killed. A different worker recovered the expired job.",
    evidence: "final-recovery-kill.json",
    id: "83f0b9b3-01bb-46ec-a382-ad6dfcb9a5e5",
  },
  {
    slug: "stale",
    title: "An obsolete result was rejected",
    description:
      "A replacement worker finished; the old attempt was later refused publication.",
    evidence: "final-recovery-stale.json",
    id: "1fa6bba1-81a4-41fe-9b29-9db482f5dea2",
  },
  {
    slug: "outage",
    title: "A bounded storage failure",
    description: "A real storage outage ended in failure after three attempts.",
    evidence: "reliability.json",
    id: "e87273e4-1c4d-4f14-b58a-f21ac540ad00",
  },
];
const sources = await Promise.all(
  definitions.map(async (def) => {
    const raw = await readJSON(`docs/evidence/${def.evidence}`);
    const job = def.slug === "outage" ? raw.storage_interruption : raw;
    assert.equal(
      job.id,
      def.id,
      "Only the reviewed synthetic jobs may be exported",
    );
    assert.equal(
      job.filename,
      def.slug === "outage" ? "reliability.mp4" : "demo.mp4",
    );
    return { def, job };
  }),
);

// Export is explicit, local-only, and never enumerates the local job list.
if (process.argv.includes("--export")) {
  const integrity = {};
  const store = async (relative, data) => {
    const destination = path.join(assets, relative);
    await mkdir(path.dirname(destination), { recursive: true });
    await writeFile(destination, data);
    integrity[relative] = { bytes: data.length, sha256: sha256(data) };
  };
  for (const { def, job } of sources.filter(
    ({ job }) => job.state === "succeeded",
  )) {
    const response = await fetch(`http://127.0.0.1:8080/v1/jobs/${job.id}`, {
      signal: AbortSignal.timeout(15000),
    });
    assert.equal(response.status, 200);
    const actual = await response.json();
    assert.equal(actual.id, job.id);
    assert.deepEqual(
      actual.metadata,
      job.metadata,
      "Export must match recorded publication",
    );
    for (const [kind, extension] of [
      ["preview", "mp4"],
      ["thumbnail", "jpg"],
      ["metadata", "json"],
    ]) {
      const output = await fetch(
        `http://127.0.0.1:8080/v1/jobs/${job.id}/outputs/${kind}`,
        { signal: AbortSignal.timeout(15000) },
      );
      assert.equal(output.status, 200);
      const data = Buffer.from(await output.arrayBuffer());
      if (kind !== "metadata")
        assert.equal(data.length, job.metadata.sizes[kind]);
      else assert.deepEqual(JSON.parse(data.toString()), job.metadata);
      // Publish safe media metadata, excluding internal object keys.
      await store(
        `media/${def.slug}/${kind}.${extension}`,
        kind === "metadata"
          ? Buffer.from(
              JSON.stringify(
                {
                  input: job.metadata.input,
                  preview: job.metadata.preview,
                  sizes: job.metadata.sizes,
                },
                null,
                2,
              ) + "\n",
            )
          : data,
      );
    }
  }
  const input = await readFile(path.join(root, ".artifacts/demo.mp4"));
  assert.equal(input.length, sources[0].job.metadata.input.size);
  // Compared to the preserved job's input_hash in PostgreSQL during CF-06 export.
  assert.equal(
    sha256(input),
    "da8bc7e3d1288b9b388bb2affeada6301823a8247889f38b229ba5b73f8fb3f2",
  );
  await store("media/input.mp4", input);
  await writeFile(
    path.join(assets, "integrity.json"),
    JSON.stringify(
      {
        implementation_revision: "d1434ab135940b6a8cdf394e747b8b53bd235400",
        assets: integrity,
      },
      null,
      2,
    ) + "\n",
  );
}

const integrity = await readJSON("web/walkthrough-public/integrity.json");
for (const [file, expected] of Object.entries(integrity.assets)) {
  const data = await readFile(path.join(assets, file));
  assert.equal(data.length, expected.bytes, `${file}: size mismatch`);
  assert.equal(sha256(data), expected.sha256, `${file}: hash mismatch`);
}
const staleLog = await readFile(
  path.join(root, "docs/evidence/final-recovery-stale.log"),
  "utf8",
);
const rejection = staleLog
  .trim()
  .split(/\r?\n/)
  .map(JSON.parse)
  .find((event) => event.category === "stale_publication");
assert.equal(rejection.job, definitions[2].id);
const recordings = sources.map(({ def, job }) => ({
  ...def,
  state: job.state,
  created_at: job.created_at,
  completed_at: job.completed_at,
  metadata: job.metadata
    ? {
        input: job.metadata.input,
        preview: job.metadata.preview,
        sizes: job.metadata.sizes,
      }
    : null,
  attempts: job.attempts.map((attempt) => ({
    number: attempt.number,
    worker: attempt.worker,
    started_at: attempt.started_at,
    ended_at: attempt.ended_at,
    outcome: attempt.outcome,
    error_category: attempt.error_category,
  })),
  rejection:
    def.slug === "stale"
      ? { time: rejection.time, category: rejection.category }
      : null,
}));
const data = {
  implementation_revision: integrity.implementation_revision,
  recordings,
  benchmarks: await readJSON(
    "docs/benchmarks/2026-09-29T06-07-55.556Z/comparison.json",
  ),
};
await writeFile(
  path.join(root, "web/walkthrough/recordings.json"),
  JSON.stringify(data, null, 2) + "\n",
);
console.log(
  `Prepared ${recordings.length} recorded scenarios; verified ${Object.keys(integrity.assets).length} public assets by SHA-256.`,
);
