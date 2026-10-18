import assert from "node:assert/strict";
import { readFile, realpath } from "node:fs/promises";

import { Compozy, CompozyUnavailableError } from "@compozy/client/node";

// The runtime E2E adapter supplies a dedicated, seeded home with an ephemeral
// listener and a unique socket. This runs against the built, public package.
const homeOption = process.env.COMPOZY_HOME;
assert.ok(homeOption, "COMPOZY_HOME must identify the isolated lifecycle home");
assert.ok(process.env.COMPOZY_BIN, "COMPOZY_BIN must identify the built CLI");
const home = await realpath(homeOption);

// An inherited remote endpoint must never redirect the lifecycle probe.
delete process.env.COMPOZY_URL;

let owner: Compozy | undefined;
try {
  owner = await Compozy.start({ home, timeoutMs: 30_000 });
  const record = JSON.parse(await readFile(`${home}/daemon.json`, "utf8")) as { pid: number };
  assert.equal(owner.daemon.pid, record.pid);
  assert.ok(record.pid > 0);

  const reused = await Compozy.start({ home, timeoutMs: 5000 });
  assert.equal(reused.daemon.pid, record.pid);
  await reused.close({ stopDaemon: true });
  await owner.close();
  const connected = await Compozy.connect({ home, timeoutMs: 5000 });
  assert.equal(
    connected.daemon.pid,
    record.pid,
    "unowned or default close must leave the daemon running"
  );

  const host = owner.daemon.httpHost;
  const authority = host.includes(":") && !host.startsWith("[") ? `[${host}]` : host;
  const baseUrl = `http://${authority}:${owner.daemon.httpPort}`;
  await owner.close({ stopDaemon: true });
  await assert.rejects(
    Compozy.connect({ baseUrl, timeoutMs: 1000 }),
    CompozyUnavailableError,
    "the owner's close must stop its daemon"
  );
  process.stdout.write(
    `${JSON.stringify({ home, pid: record.pid, reused: true, stopped: true })}\n`
  );
} finally {
  await owner?.close({ stopDaemon: true });
}
