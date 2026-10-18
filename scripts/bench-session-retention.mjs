import { heapStats } from "bun:jsc";
import { createHash } from "node:crypto";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

// Run with Bun, passing a real session-thread-repository module, messages per
// session, and session count. Each variant must run in a fresh process.
const [modulePath, countInput = "2000", sessionsInput = "1", maxBytesInput] = process.argv.slice(2);
if (!modulePath) throw new Error("Pass the repository module path");
const { toReadonlyThreadMessages } = await import(pathToFileURL(resolve(modulePath)).href);
const count = Number(countInput);
const sessions = Number(sessionsInput);
const maxRetainedBytes = maxBytesInput === undefined ? undefined : Number(maxBytesInput);
if (!Number.isInteger(count) || count < 1 || !Number.isInteger(sessions) || sessions < 1) {
  throw new Error("Message and session counts must be positive integers");
}
if (
  maxRetainedBytes !== undefined &&
  (!Number.isFinite(maxRetainedBytes) || maxRetainedBytes < 0)
) {
  throw new Error("Retained-byte budget must be a nonnegative finite number");
}
const yieldJob = () => new Promise(resolve => setTimeout(resolve, 0));
async function collect() {
  await yieldJob();
  Bun.gc(true);
  await yieldJob();
  Bun.gc(true);
  return heapStats().heapSize;
}

toReadonlyThreadMessages([
  { id: "warm", role: "assistant", parts: [{ type: "text", text: "warm" }] },
]);
const before = await collect();
const histories = Array.from({ length: sessions }, (_, session) =>
  Array.from({ length: count }, (_, index) => ({
    id: `${session}-${index}`,
    role: index % 2 ? "assistant" : "user",
    parts: [{ type: "text", text: `Message ${session}-${index}` }],
  }))
);
let latest = [];
const start = performance.now();
for (let index = 1; index <= count; index += 1) {
  latest = histories.map(history => toReadonlyThreadMessages(history.slice(0, index)));
  // Streaming deliveries occur on separate jobs. Yield so the previous job's
  // WeakRef targets can become collectible, retaining only the current views.
  if (index % 100 === 0) await yieldJob();
}
const elapsedMs = performance.now() - start;
const after = await collect();
const retainedBytes = after - before;
const stable = histories.every(
  (history, index) => toReadonlyThreadMessages(history.slice()) === latest[index]
);
const digest = createHash("sha256")
  .update(
    JSON.stringify(
      latest.map(messages =>
        messages.map(({ id, role, content, status, metadata }) => ({
          id,
          role,
          content,
          status,
          metadata,
        }))
      )
    )
  )
  .digest("hex");
console.log(
  JSON.stringify({
    runtime: `Bun ${Bun.version}`,
    modulePath,
    count,
    sessions,
    before,
    after,
    retainedBytes,
    elapsedMs,
    stable,
    digest,
    liveMessages: histories.reduce((sum, history) => sum + history.length, 0),
    latestViews: latest.length,
  })
);
if (!stable || (maxRetainedBytes !== undefined && retainedBytes > maxRetainedBytes)) {
  process.exitCode = 1;
}
