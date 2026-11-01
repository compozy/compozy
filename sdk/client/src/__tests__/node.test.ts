import { chmod, mkdir, mkdtemp, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { createServer, type AddressInfo } from "node:net";
import { homedir, tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  CompozyBinaryNotFoundError,
  CompozyConfigError,
  CompozyUnavailableError,
} from "../errors.js";
import { Compozy as NodeClient } from "../index-node.js";
import { resolveBinary } from "../node-binary.js";
import { discover } from "../node-discovery.js";
import { Compozy } from "../node.js";

// Invariants: deterministic binary discovery, home/PID ownership, and bounded
// startup cleanup and completed shutdown. Owner: Node lifecycle; canonical suite: this file. The CLI
// fixture is an I/O boundary and uses actual processes, HTTP, and daemon records.
const fixture = `#!${process.execPath}
const fs = require("node:fs");
const path = require("node:path");
const http = require("node:http");
const home = process.env.COMPOZY_HOME;
const file = name => path.join(home, name);
const options = fs.existsSync(file("fixture.json")) ? JSON.parse(fs.readFileSync(file("fixture.json"), "utf8")) : {};
if (process.argv[3] === "stop") {
  fs.appendFileSync(file("stops"), "stop\\n");
  if (options.stopReplacementInfo) fs.writeFileSync(file("daemon.json"), JSON.stringify(options.stopReplacementInfo));
  const info = JSON.parse(fs.readFileSync(file("daemon.json"), "utf8"));
  process.kill(info.pid, "SIGTERM");
  if (options.stopAckEarly) process.exit(0);
  const timer = setInterval(() => {
    try { process.kill(info.pid, 0); } catch { clearInterval(timer); process.exit(0); }
  }, 10);
} else {
  fs.writeFileSync(file("started.json"), JSON.stringify({ pid: process.pid, args: process.argv.slice(2), url: process.env.COMPOZY_URL }));
  if (options.neverReady) {
    process.stderr.write("test initialization blocked\\n");
    if (options.ignoreTerm) process.on("SIGTERM", () => {});
    setInterval(() => {}, 1000);
  } else {
    const server = http.createServer((req, res) => {
      if (req.url !== "/api/status/identity") { res.writeHead(404).end(); return; }
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify({ daemon: {
        version: "0.0.0-dev", http_host: "127.0.0.1", http_port: server.address().port,
        pid: process.pid + (options.pidOffset || 0), user_home_dir: require("node:os").homedir(),
        started_at: new Date().toISOString()
      }}));
    });
    server.listen(0, "127.0.0.1", () => {
      // Publish like the real daemon: a poller must never read a partial record.
      fs.writeFileSync(file("daemon.json.tmp"), JSON.stringify({ pid: process.pid, port: server.address().port, http_host: "127.0.0.1" }));
      fs.renameSync(file("daemon.json.tmp"), file("daemon.json"));
    });
    process.on("SIGTERM", () => {
      fs.appendFileSync(file("terminations"), "SIGTERM\\n");
      try {
        if (JSON.parse(fs.readFileSync(file("daemon.json"), "utf8")).pid === process.pid) fs.unlinkSync(file("daemon.json"));
      } catch {}
      const finish = () => {
        server.close(() => process.exit(0));
        server.closeAllConnections();
      };
      if (options.stopAckEarly) setTimeout(finish, 150);
      else finish();
    });
  }
}
`;

let root: string;
let homes: string[];

beforeEach(async () => {
  root = await realpath(await mkdtemp(join(tmpdir(), "compozy-sdk-node-")));
  homes = [];
  vi.stubEnv("COMPOZY_BIN", "");
  vi.stubEnv("COMPOZY_URL", "");
  vi.stubEnv("COMPOZY_APP_TOKEN", "");
  vi.stubEnv("COMPOZY_HOME", join(root, "default-home"));
});

afterEach(async () => {
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  for (const home of homes) {
    try {
      const { pid } = JSON.parse(await readFile(join(home, "started.json"), "utf8")) as {
        pid: number;
      };
      if (alive(pid)) {
        process.kill(pid, "SIGKILL");
        await vi.waitFor(() => expect(alive(pid)).toBe(false));
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
    }
  }
  await rm(root, { recursive: true, force: true });
});

function alive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

async function binary(path = join(root, "compozy")): Promise<string> {
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, fixture);
  await chmod(path, 0o700);
  return path;
}

async function home(options: Record<string, unknown> = {}): Promise<string> {
  const directory = join(root, `home-${homes.length}`);
  homes.push(directory);
  await mkdir(directory);
  await writeFile(join(directory, "fixture.json"), JSON.stringify(options));
  return directory;
}

async function started(directory: string): Promise<{ pid: number; args: string[]; url?: string }> {
  return JSON.parse(await readFile(join(directory, "started.json"), "utf8"));
}

describe("Node binary lookup", () => {
  // Invariant: installed CLI discovery follows the application, independently
  // of its caller's working directory. Owner and canonical suite: Node lookup.
  it("resolves the application's installed CLI when launched from another directory", async () => {
    const name = process.platform === "win32" ? "compozy.exe" : "compozy";
    const app = join(root, "app");
    const elsewhere = join(root, "elsewhere");
    const installed = await binary(join(app, "node_modules", "@compozy", "cli", "bin", name));
    const cwdInstalled = await binary(
      join(elsewhere, "node_modules", "@compozy", "cli", "bin", name)
    );
    for (const directory of [app, elsewhere]) {
      await writeFile(
        join(directory, "node_modules", "@compozy", "cli", "package.json"),
        '{"name":"@compozy/cli","version":"0.0.0"}'
      );
    }
    vi.spyOn(process, "cwd").mockReturnValue(elsewhere);
    vi.stubEnv("PATH", "");
    const argv = process.argv;
    try {
      process.argv = [process.execPath, join(app, "run.mjs")];
      expect(await resolveBinary()).toBe(installed);
      process.argv = [process.execPath];
      expect(await resolveBinary()).toBe(cwdInstalled);
    } finally {
      process.argv = argv;
    }
  });

  it("uses the explicit binary, environment, installed CLI, then PATH", async () => {
    const explicit = await binary(join(root, "explicit"));
    const environment = await binary(join(root, "environment"));
    const name = process.platform === "win32" ? "compozy.exe" : "compozy";
    const installed = await binary(join(root, "node_modules", "@compozy", "cli", "bin", name));
    await writeFile(
      join(root, "node_modules", "@compozy", "cli", "package.json"),
      '{"name":"@compozy/cli","version":"0.0.0"}'
    );
    const path = await binary(join(root, "bin", name));
    vi.spyOn(process, "cwd").mockReturnValue(root);
    vi.stubEnv("COMPOZY_BIN", environment);
    vi.stubEnv("PATH", dirname(path));
    expect(await resolveBinary(explicit)).toBe(explicit);
    expect(await resolveBinary()).toBe(environment);
    vi.stubEnv("COMPOZY_BIN", "");
    expect(await resolveBinary()).toBe(installed);
    await rm(installed);
    expect(await resolveBinary()).toBe(path);
    await rm(path);
    const error = await resolveBinary().catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(CompozyBinaryNotFoundError);
    expect((error as Error).message).toMatch(/binaryPath option, COMPOZY_BIN, @compozy\/cli, PATH/);
  });
});

describe("Node runtime support", () => {
  // Invariant: both public prompt methods generate independent wire IDs without
  // browser globals. Owner: Node entry point; canonical suite: this file.
  it("generates prompt IDs without a global crypto implementation", async () => {
    vi.stubGlobal("crypto", undefined);
    const requests: Array<{ message_id: unknown; idempotency_key: unknown }> = [];
    const transport: typeof fetch = async (input, init) => {
      const request = new Request(input, init);
      if (request.url.endsWith("/api/status/identity")) {
        return Response.json({
          daemon: { pid: 101, version: "0.0.0-dev", http_host: "127.0.0.1", http_port: 2123 },
        });
      }
      const body = (await request.json()) as { message_id: unknown; idempotency_key: unknown };
      requests.push(body);
      return Response.json({ prompt: { ...body, status: "queued" } }, { status: 202 });
    };
    const client = await NodeClient.connect({ baseUrl: "http://127.0.0.1:2123", fetch: transport });
    const input = { workspaceId: "workspace", sessionId: "session", message: "hello" };
    expect((await client.prompt(input)).kind).toBe("accepted");
    expect((await client.promptResponse(input)).status).toBe(202);
    expect(requests).toHaveLength(2);
    for (const request of requests) {
      expect(request.idempotency_key).toEqual(expect.any(String));
      expect(request.idempotency_key).not.toBe("");
      expect(request.message_id).toBe(request.idempotency_key);
    }
    expect(requests[0]!.idempotency_key).not.toBe(requests[1]!.idempotency_key);
  });
});

describe("Node discovery", () => {
  // Invariant: a Node connection chooses one documented source and validates it.
  // Owner: Node discovery; canonical suite: this Node entry-point suite.
  it("prefers explicit URL, environment URL, explicit home, environment home, then the default port", async () => {
    const explicitHome = await home();
    const environmentHome = await home();
    await writeFile(join(explicitHome, "daemon.json"), JSON.stringify({ pid: 10, port: 3210 }));
    await writeFile(join(environmentHome, "daemon.json"), JSON.stringify({ pid: 20, port: 3220 }));
    vi.stubEnv("COMPOZY_HOME", environmentHome);
    vi.stubEnv("COMPOZY_URL", "http://127.0.0.1:3230");
    expect(await discover({ baseUrl: "http://127.0.0.1:3240", home: explicitHome })).toMatchObject({
      baseUrl: "http://127.0.0.1:3240",
    });
    expect(await discover({ home: explicitHome })).toMatchObject({
      baseUrl: "http://127.0.0.1:3230",
    });
    vi.stubEnv("COMPOZY_URL", "");
    expect(await discover({ home: explicitHome })).toMatchObject({
      baseUrl: "http://127.0.0.1:3210",
    });
    expect(await discover()).toMatchObject({ baseUrl: "http://127.0.0.1:3220" });
    vi.stubEnv("COMPOZY_HOME", join(root, "missing-home"));
    expect(await discover()).toMatchObject({ baseUrl: "http://127.0.0.1:2123" });
  });

  it("uses the stored host and defaults older records to IPv4 loopback", async () => {
    const directory = await home();
    const record = join(directory, "daemon.json");
    await writeFile(record, JSON.stringify({ pid: 10, port: 3210, http_host: "::1" }));
    expect(await discover({ home: directory })).toMatchObject({ baseUrl: "http://[::1]:3210" });
    await writeFile(record, JSON.stringify({ pid: 10, port: 3210 }));
    expect(await discover({ home: directory })).toMatchObject({ baseUrl: "http://127.0.0.1:3210" });
  });

  it("rejects an invalid environment URL without falling through to another daemon", async () => {
    vi.stubEnv("COMPOZY_URL", "localhost:abc");
    const connection = Compozy.connect();
    await expect(connection).rejects.toBeInstanceOf(CompozyConfigError);
    await expect(connection).rejects.toThrow("COMPOZY_URL");
  });

  it("rejects an unreachable stale record within the connection deadline", async () => {
    const listener = createServer();
    await new Promise<void>((resolveListen, reject) => {
      listener.once("error", reject);
      listener.listen(0, "127.0.0.1", resolveListen);
    });
    const port = (listener.address() as AddressInfo).port;
    await new Promise<void>((resolveClose, reject) =>
      listener.close(error => (error ? reject(error) : resolveClose()))
    );
    const directory = await home();
    await writeFile(join(directory, "daemon.json"), JSON.stringify({ pid: 10, port }));
    const before = Date.now();
    const connection = Compozy.connect({ home: directory, timeoutMs: 200 });
    await expect(connection).rejects.toBeInstanceOf(CompozyUnavailableError);
    await expect(connection).rejects.toThrow("compozy daemon start");
    expect(Date.now() - before).toBeLessThan(1000);
  });
});

describe.skipIf(process.platform === "win32")("Node daemon lifecycle", () => {
  it.each([false, true])(
    "starts one home, reuses its daemon, and only the owner can stop it (delayed exit: %s)",
    async stopAckEarly => {
      const directory = await home({ stopAckEarly });
      vi.stubEnv("COMPOZY_URL", "http://127.0.0.1:1");
      const owner = await Compozy.start({
        home: directory,
        binaryPath: await binary(),
        timeoutMs: 3000,
      });
      const launch = await started(directory);
      expect(launch.args).toEqual(["daemon", "start", "--foreground"]);
      expect(launch.url).toBeUndefined();
      expect(owner.daemon.pid).toBe(launch.pid);
      expect(owner.daemon.userHomeDir).toBe(homedir());
      expect(owner.daemon.userHomeDir).not.toBe(directory);
      const reused = await Compozy.start({ home: directory, binaryPath: "/missing/compozy" });
      await reused.close({ stopDaemon: true });
      await owner.close();
      expect(alive(launch.pid)).toBe(true);
      await expect(readFile(join(directory, "stops"))).rejects.toMatchObject({ code: "ENOENT" });
      await Promise.all([owner.close({ stopDaemon: true }), owner.close({ stopDaemon: true })]);
      expect(alive(launch.pid)).toBe(false);
      expect(await readFile(join(directory, "terminations"), "utf8")).toBe("SIGTERM\n");
      await expect(readFile(join(directory, "stops"))).rejects.toMatchObject({ code: "ENOENT" });
      await owner.close({ stopDaemon: true });
    }
  );

  it("signals the original process without a second home discovery that could stop a replacement", async () => {
    const directory = await home();
    const replacementHome = await home();
    const binaryPath = await binary();
    const owner = await Compozy.start({ home: directory, binaryPath, timeoutMs: 3000 });
    const replacement = await Compozy.start({ home: replacementHome, binaryPath, timeoutMs: 3000 });
    const original = await started(directory);
    const replacementInfo = JSON.parse(
      await readFile(join(replacementHome, "daemon.json"), "utf8")
    );
    // The CLI I/O fixture replaces the home record immediately before its own
    // PID read, making the old check-then-discover race deterministic.
    await writeFile(
      join(directory, "fixture.json"),
      JSON.stringify({ stopReplacementInfo: replacementInfo })
    );
    await owner.close({ stopDaemon: true });
    expect(alive(original.pid)).toBe(false);
    expect(alive(replacementInfo.pid)).toBe(true);
    expect(await readFile(join(directory, "terminations"), "utf8")).toBe("SIGTERM\n");
    await expect(readFile(join(directory, "stops"))).rejects.toMatchObject({ code: "ENOENT" });
    await replacement.close({ stopDaemon: true });
  });

  it("does not stop a daemon after its home records a different PID", async () => {
    const directory = await home();
    const owner = await Compozy.start({
      home: directory,
      binaryPath: await binary(),
      timeoutMs: 3000,
    });
    const recordPath = join(directory, "daemon.json");
    const record = JSON.parse(await readFile(recordPath, "utf8")) as { pid: number; port: number };
    await writeFile(recordPath, JSON.stringify({ ...record, pid: record.pid + 100000 }));
    await owner.close({ stopDaemon: true });
    expect(alive(record.pid)).toBe(true);
    await expect(readFile(join(directory, "stops"))).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("rejects a foreign runtime PID and reaps only the child it launched", async () => {
    const directory = await home({ pidOffset: 1 });
    await expect(
      Compozy.start({ home: directory, binaryPath: await binary(), timeoutMs: 3000 })
    ).rejects.toBeInstanceOf(CompozyConfigError);
    expect(alive((await started(directory)).pid)).toBe(false);
  });

  it("includes current stderr and reaps a startup that never publishes daemon.json", async () => {
    const directory = await home({ neverReady: true, ignoreTerm: true });
    await mkdir(join(directory, "logs"));
    await writeFile(join(directory, "logs", "daemon-stdio.log"), "old startup error\n");
    const before = performance.now();
    const startupAt = Date.now();
    // Control the deadline until the real child has emitted the stderr being tested.
    const clock = vi.spyOn(Date, "now").mockReturnValue(startupAt);
    const startup = Compozy.start({
      home: directory,
      binaryPath: await binary(),
      timeoutMs: 300,
    }).catch((cause: unknown) => cause);
    try {
      await expect
        .poll(() => readFile(join(directory, "logs", "daemon-stdio.log"), "utf8"), {
          timeout: 2500,
        })
        .toContain("test initialization blocked");
      clock.mockReturnValue(startupAt + 300);
      const error = await startup;
      expect(error).toBeInstanceOf(CompozyUnavailableError);
      expect((error as Error).message).toContain("test initialization blocked");
      expect((error as Error).message).not.toContain("old startup error");
      expect(performance.now() - before).toBeLessThan(2500);
      expect(alive((await started(directory)).pid)).toBe(false);
    } finally {
      clock.mockRestore();
      await startup;
    }
  });

  it("rejects invalid homes and deadlines before spawning", async () => {
    const path = join(root, "file");
    await writeFile(path, "not a directory");
    await expect(Compozy.start({ home: path })).rejects.toBeInstanceOf(CompozyConfigError);
    await expect(Compozy.start({ home: " " })).rejects.toBeInstanceOf(CompozyConfigError);
    await expect(Compozy.start({ home: root, timeoutMs: 0 })).rejects.toBeInstanceOf(
      CompozyConfigError
    );
  });
});
