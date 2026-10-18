import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";

import { Compozy, CompozyApiError, type PromptReply, type UIMessageChunk } from "@compozy/client";

// Invariants: the published SDK drives real sessions, replay, queued input, proxy
// disconnects, and cancel. Owner: daemon/client integration; canonical suite:
// TestDaemonE2EAppSDK. ACP mock fixtures are supplied by that runtime harness.
const workspacePath = required("COMPOZY_TEST_WORKSPACE");
const agent = required("COMPOZY_TEST_AGENT");
const expectedReply = required("COMPOZY_TEST_REPLY");
const signal = AbortSignal.timeout(60_000);
const compozy = await Compozy.connect();
const { workspace } = await compozy.workspaces.resolveWorkspace({
  body: { path: workspacePath },
  signal,
});

function required(name: string): string {
  const value = process.env[name];
  assert.ok(value, `${name} is required`);
  return value;
}

async function session() {
  const created = await compozy.sessions.createSession({
    body: { workspace: workspace.id, agent_name: agent },
    signal,
  });
  const path = { workspace_id: workspace.id, session_id: created.session.id };
  await until(
    async () => (await compozy.sessions.getSession({ path, signal })).session.state === "active"
  );
  return { path, workspaceId: workspace.id, sessionId: created.session.id };
}

async function until(predicate: () => Promise<boolean>): Promise<void> {
  while (!(await predicate())) {
    signal.throwIfAborted();
    await new Promise(resolve => setTimeout(resolve, 25));
  }
}

async function consume(reply: PromptReply): Promise<UIMessageChunk[]> {
  assert.equal(reply.kind, "stream");
  if (reply.kind !== "stream") throw new Error("Expected a streamed prompt");
  const chunks = [];
  for await (const chunk of reply.chunks) chunks.push(chunk);
  assert.ok(
    chunks.some(chunk => chunk.type === "finish"),
    "prompt must finish"
  );
  return chunks;
}

function streamController(): AbortController {
  const controller = new AbortController();
  signal.addEventListener("abort", () => controller.abort(), { once: true });
  if (signal.aborted) controller.abort();
  return controller;
}

async function idle(target: Awaited<ReturnType<typeof session>>) {
  await until(
    async () =>
      !(await compozy.sessions.getSessionHealth({ path: target.path, signal })).health.active_prompt
  );
}

const golden = await session();
const chunks = await consume(await compozy.prompt({ ...golden, message: "sdk golden", signal }));
assert.ok(chunks.some(chunk => chunk.type === "start"));
assert.ok(
  chunks
    .flatMap(chunk => (chunk.type === "text-delta" ? [chunk.delta] : []))
    .join("")
    .includes(expectedReply)
);
await idle(golden);
await assert.rejects(
  compozy.sessions.getSession({
    path: { ...golden.path, session_id: "sess-sdk-does-not-exist" },
    signal,
  }),
  error =>
    error instanceof CompozyApiError && error.status === 404 && error.code === "session_not_found"
);

// Default transcript feeds expose named frames; raw feeds provide durable cursors.
const snapshotAbort = streamController();
let cursor = 0;
for await (const frame of compozy.events("streamSession", {
  path: golden.path,
  signal: snapshotAbort.signal,
})) {
  if (frame.event !== "transcript_snapshot") continue;
  assert.ok(frame.data && typeof frame.data === "object");
  cursor = Number(frame.id);
  snapshotAbort.abort();
  break;
}
signal.throwIfAborted();
assert.ok(cursor > 0, "transcript snapshot must carry its replay cursor");
await consume(await compozy.prompt({ ...golden, message: "sdk replay", signal }));
await idle(golden);
const after = (await compozy.sessions.getSessionTranscript({ path: golden.path, signal }))
  .max_sequence;
assert.ok(after > cursor);
const replayAbort = streamController();
const seen = new Set<number>();
for await (const frame of compozy.events("streamSession", {
  path: golden.path,
  query: { frames: "raw", after_sequence: cursor, limit: 1000 },
  signal: replayAbort.signal,
})) {
  if (!frame.id) continue;
  const sequence = Number(frame.id);
  assert.ok(sequence > cursor, `replayed cursor ${sequence} must follow ${cursor}`);
  assert.ok(!seen.has(sequence), `duplicate event ${sequence}`);
  seen.add(sequence);
  if (sequence >= after) {
    replayAbort.abort();
    break;
  }
}
signal.throwIfAborted();
assert.ok(seen.size > 0);

const busy = await session();
const first = await compozy.prompt({ ...busy, message: "sdk busy", signal });
assert.equal(first.kind, "stream");
if (first.kind !== "stream") throw new Error("Expected busy stream");
const firstIterator = first.chunks[Symbol.asyncIterator]();
while (true) {
  const next = await firstIterator.next();
  assert.equal(next.done, false, "busy turn must emit text before completion");
  if (next.value.type === "text-delta") break;
  signal.throwIfAborted();
}
const queued = await compozy.prompt({
  ...busy,
  message: "sdk queued",
  mode: "queue",
  idempotencyKey: "sdk-queue-once",
  signal,
});
assert.equal(queued.kind, "accepted");
if (queued.kind === "accepted") {
  assert.equal(queued.result.status, "queued");
  assert.equal(queued.result.queue_position, 1);
  const replayed = await compozy.prompt({
    ...busy,
    message: "sdk queued",
    mode: "queue",
    idempotencyKey: "sdk-queue-once",
    signal,
  });
  assert.equal(replayed.kind, "accepted");
  if (replayed.kind === "accepted") {
    assert.equal(replayed.result.replayed, true);
    assert.equal(replayed.result.queue_entry_id, queued.result.queue_entry_id);
  }
}
while (!(await firstIterator.next()).done) signal.throwIfAborted();

const proxied = await session();
const server = createServer(async (_request, response) => {
  const disconnect = new AbortController();
  response.once("close", () => disconnect.abort());
  try {
    const upstream = await compozy.promptResponse({
      ...proxied,
      message: "sdk proxy",
      signal: disconnect.signal,
    });
    response.writeHead(upstream.status, Object.fromEntries(upstream.headers));
    assert.ok(upstream.body);
    const reader = upstream.body.getReader();
    try {
      while (!disconnect.signal.aborted) {
        const next = await reader.read();
        if (next.done) break;
        response.write(next.value);
      }
      response.end();
    } finally {
      await reader.cancel().catch(() => {});
    }
  } catch (error) {
    if (!disconnect.signal.aborted)
      response.destroy(error instanceof Error ? error : new Error(String(error)));
  }
});
server.listen(0, "127.0.0.1");
await once(server, "listening");
try {
  const address = server.address();
  assert.ok(address && typeof address === "object");
  const downstream = await fetch(`http://127.0.0.1:${address.port}`, { signal });
  assert.equal(downstream.status, 200);
  assert.equal(downstream.headers.get("x-vercel-ai-ui-message-stream"), "v1");
  assert.ok(downstream.body);
  const reader = downstream.body.getReader();
  assert.equal((await reader.read()).done, false);
  await reader.cancel();
  await until(async () => {
    const transcript = await compozy.sessions.getSessionTranscript({ path: proxied.path, signal });
    return JSON.stringify(transcript.entries).includes(expectedReply);
  });
  await idle(proxied);
  const completed = await compozy.sessions.getSessionHistory({ path: proxied.path, signal });
  assert.ok(
    completed.history
      .flatMap(turn => turn.events)
      .some(
        event =>
          event.type === "done" &&
          event.content &&
          typeof event.content === "object" &&
          "prompt_stop_reason" in event.content &&
          event.content.prompt_stop_reason === "end_turn"
      ),
    "disconnected proxy must leave a completed daemon turn"
  );
} finally {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close(error => (error ? reject(error) : resolve()))
  );
}

const canceled = await session();
const cancelReply = await compozy.prompt({ ...canceled, message: "sdk cancel", signal });
assert.equal(cancelReply.kind, "stream");
if (cancelReply.kind !== "stream") throw new Error("Expected cancelable stream");
let cancelSent = false;
let canceledTurnId: string | undefined;
let completed = false;
for await (const chunk of cancelReply.chunks) {
  if (chunk.type === "text-delta" && !cancelSent) {
    cancelSent = true;
    const result = await compozy.sessions.cancelSessionPrompt({ path: canceled.path, signal });
    assert.equal(result.outcome, "canceled");
    assert.ok(result.turn_id);
    canceledTurnId = result.turn_id;
  }
  if (chunk.type === "finish") completed = true;
}
assert.ok(cancelSent && completed, "cancel must terminate the streamed turn");
await idle(canceled);
await until(async () => {
  const { history } = await compozy.sessions.getSessionHistory({
    path: canceled.path,
    query: { turn_id: canceledTurnId },
    signal,
  });
  return history
    .flatMap(turn => turn.events)
    .some(
      event =>
        event.type === "done" &&
        event.content &&
        typeof event.content === "object" &&
        "prompt_stop_reason" in event.content &&
        event.content.prompt_stop_reason === "cancelled"
    );
});
console.log(
  JSON.stringify({
    scenario: "app-sdk-golden-path",
    workspace: workspace.id,
    reply: expectedReply,
    replayed: seen.size,
    queued: true,
    detached: true,
    canceled: true,
  })
);
