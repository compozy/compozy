import { execFileSync, spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { mkdtemp, readdir, rm, stat, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import readline from "node:readline";

import { afterAll, beforeAll, describe, expect, it } from "vitest";

let tempDirs: string[] = [];
const sourceDir = resolve(__dirname, "..");
const packageDir = resolve(sourceDir, "..");
const workspaceRoot = resolve(packageDir, "../..");
const tscBin = join(workspaceRoot, "node_modules/.bin/tsc");
const buildTimeoutMs = 120_000;
const integrationTimeoutMs = 180_000;
const messageTimeoutMs = 5_000;

async function newestMtimeMs(path: string): Promise<number> {
  const entry = await stat(path);
  if (!entry.isDirectory()) {
    return entry.mtimeMs;
  }

  let newest = entry.mtimeMs;
  for (const child of await readdir(path, { withFileTypes: true })) {
    newest = Math.max(newest, await newestMtimeMs(join(path, child.name)));
  }

  return newest;
}

async function oldestMtimeMs(paths: string[]): Promise<number> {
  let oldest = Number.POSITIVE_INFINITY;

  for (const path of paths) {
    oldest = Math.min(oldest, (await stat(path)).mtimeMs);
  }

  return oldest;
}

async function sdkBuildIsCurrent(): Promise<boolean> {
  const outputs = [
    join(packageDir, "dist/esm/index.js"),
    join(packageDir, "dist/cjs/index.js"),
    join(packageDir, "dist/types/index.d.ts"),
    join(packageDir, "dist/esm/package.json"),
    join(packageDir, "dist/cjs/package.json"),
  ];

  try {
    const newestInput = Math.max(
      await newestMtimeMs(join(packageDir, "src")),
      await newestMtimeMs(join(packageDir, "package.json")),
      await newestMtimeMs(join(packageDir, "tsconfig.types.json")),
      await newestMtimeMs(join(packageDir, "tsconfig.esm.json")),
      await newestMtimeMs(join(packageDir, "tsconfig.cjs.json")),
      await newestMtimeMs(join(packageDir, "scripts/postbuild.mjs"))
    );
    const oldestOutput = await oldestMtimeMs(outputs);

    return oldestOutput >= newestInput;
  } catch {
    return false;
  }
}

async function buildSDK(): Promise<void> {
  execFileSync(tscBin, ["-p", join(packageDir, "tsconfig.types.json")], {
    cwd: packageDir,
    stdio: "pipe",
  });
  execFileSync(tscBin, ["-p", join(packageDir, "tsconfig.esm.json")], {
    cwd: packageDir,
    stdio: "pipe",
  });
  execFileSync(tscBin, ["-p", join(packageDir, "tsconfig.cjs.json")], {
    cwd: packageDir,
    stdio: "pipe",
  });
  execFileSync(process.execPath, [join(packageDir, "scripts/postbuild.mjs")], {
    cwd: packageDir,
    stdio: "pipe",
  });
}

async function ensureBuiltSDK(): Promise<void> {
  if (await sdkBuildIsCurrent()) {
    return;
  }

  await buildSDK();
}

interface ProcessDiagnostics {
  child: ChildProcessWithoutNullStreams;
  label: string;
  stderrLines?: string[];
  timeoutMs?: number;
}

function formatDiagnostics({ child, label, stderrLines = [] }: ProcessDiagnostics): string {
  const exitDetail =
    child.exitCode === null && child.signalCode === null
      ? "still running"
      : `exit=${child.exitCode ?? "null"} signal=${child.signalCode ?? "null"}`;
  const stderrTail = stderrLines.slice(-20).join("\n");
  return [
    `while waiting for ${label}`,
    `child ${exitDetail}`,
    stderrTail ? `stderr:\n${stderrTail}` : "stderr: <empty>",
  ].join("; ");
}

interface LineWaiter {
  accept: (line: string) => void;
}

class BufferedMessageReader {
  private readonly queuedLines: string[] = [];
  private readonly waiters: LineWaiter[] = [];
  private closed = false;

  public constructor(private readonly rl: readline.Interface) {
    this.rl.on("line", line => {
      const waiter = this.waiters.shift();
      if (waiter) {
        waiter.accept(line);
        return;
      }
      this.queuedLines.push(line);
    });
    this.rl.on("close", () => {
      this.closed = true;
    });
  }

  public async next(diagnostics: ProcessDiagnostics): Promise<Record<string, unknown>> {
    const line = await this.nextLine(diagnostics);
    try {
      return JSON.parse(line) as Record<string, unknown>;
    } catch (error) {
      throw error instanceof Error
        ? new Error(`invalid JSON ${formatDiagnostics(diagnostics)}: ${error.message}`)
        : new Error(`invalid JSON ${formatDiagnostics(diagnostics)}: ${String(error)}`);
    }
  }

  private async nextLine(diagnostics: ProcessDiagnostics): Promise<string> {
    const queued = this.queuedLines.shift();
    if (queued !== undefined) {
      return queued;
    }
    if (this.closed) {
      throw new Error(`stdout closed ${formatDiagnostics(diagnostics)}`);
    }

    return await new Promise((resolve, reject) => {
      const waiter: LineWaiter = {
        accept: line => {
          cleanup();
          resolve(line);
        },
      };
      const timeout = setTimeout(() => {
        cleanup();
        reject(new Error(`timed out ${formatDiagnostics(diagnostics)}`));
      }, diagnostics.timeoutMs ?? messageTimeoutMs);

      const cleanup = (): void => {
        clearTimeout(timeout);
        const waiterIndex = this.waiters.indexOf(waiter);
        if (waiterIndex !== -1) {
          this.waiters.splice(waiterIndex, 1);
        }
        this.rl.off("close", onClose);
        diagnostics.child.off("exit", onExit);
        diagnostics.child.off("error", onError);
      };
      const onClose = (): void => {
        cleanup();
        reject(new Error(`stdout closed ${formatDiagnostics(diagnostics)}`));
      };
      const onExit = (): void => {
        cleanup();
        reject(new Error(`child exited ${formatDiagnostics(diagnostics)}`));
      };
      const onError = (error: Error): void => {
        cleanup();
        reject(
          new Error(`child process error ${formatDiagnostics(diagnostics)}: ${error.message}`)
        );
      };

      this.waiters.push(waiter);
      this.rl.once("close", onClose);
      diagnostics.child.once("exit", onExit);
      diagnostics.child.once("error", onError);
    });
  }
}

async function waitForStderrLine(
  rl: readline.Interface,
  diagnostics: ProcessDiagnostics,
  pattern: RegExp
): Promise<void> {
  if ((diagnostics.stderrLines ?? []).some(line => pattern.test(line))) {
    return;
  }

  await new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup();
      reject(new Error(`timed out ${formatDiagnostics(diagnostics)}`));
    }, diagnostics.timeoutMs ?? messageTimeoutMs);

    const cleanup = (): void => {
      clearTimeout(timeout);
      rl.off("line", onLine);
      rl.off("close", onClose);
      diagnostics.child.off("exit", onExit);
      diagnostics.child.off("error", onError);
    };
    const onLine = (line: string): void => {
      if (!pattern.test(line)) {
        return;
      }
      cleanup();
      resolve();
    };
    const onClose = (): void => {
      cleanup();
      reject(new Error(`stderr closed ${formatDiagnostics(diagnostics)}`));
    };
    const onExit = (): void => {
      cleanup();
      reject(new Error(`child exited ${formatDiagnostics(diagnostics)}`));
    };
    const onError = (error: Error): void => {
      cleanup();
      reject(new Error(`child process error ${formatDiagnostics(diagnostics)}: ${error.message}`));
    };

    rl.on("line", onLine);
    rl.once("close", onClose);
    diagnostics.child.once("exit", onExit);
    diagnostics.child.once("error", onError);
  });
}

async function terminateChild(
  child: ChildProcessWithoutNullStreams,
  interfaces: readline.Interface[]
): Promise<void> {
  for (const rl of interfaces) {
    rl.close();
  }
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }

  const exited = new Promise<void>(resolve => {
    child.once("exit", () => resolve());
  });
  child.kill();
  await Promise.race([exited, new Promise(resolve => setTimeout(resolve, 1_000))]);
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }
  child.kill("SIGKILL");
  await Promise.race([exited, new Promise(resolve => setTimeout(resolve, 1_000))]);
}

describe("SDK integration", () => {
  beforeAll(async () => {
    await ensureBuiltSDK();
  }, buildTimeoutMs);

  afterAll(async () => {
    await Promise.all(tempDirs.map(async dir => await rm(dir, { force: true, recursive: true })));
  });

  it(
    "builds an SDK-based extension and serves real JSON-RPC over stdio",
    async () => {
      const sdkEntry = resolve(packageDir, "dist/esm/index.js");
      const tempDir = await mkdtemp(join(tmpdir(), "agh-sdk-integration-"));
      tempDirs.push(tempDir);
      await writeFile(
        join(tempDir, "index.mjs"),
        `import { Extension } from ${JSON.stringify(sdkEntry)};
       const extension = new Extension(
         {
           name: "integration-ext",
           version: "0.1.0",
           capabilities: { provides: ["memory.backend"] },
           actions: { requires: ["sessions/list"] },
           security: { capabilities: ["memory.read", "memory.write", "session.read"] }
         }
       );
       extension.handle("memory/store", async (_ctx, params) => ({ stored: params.key }));
       extension.handle("memory/recall", async () => ({ entries: [] }));
       extension.handle("memory/forget", async () => ({}));
       extension.handle("health_check", async () => ({ healthy: true, message: "", details: {} }));
       extension.onReady(async (host) => {
         const sessions = await host.sessions.list();
         console.error("ready sessions", sessions.length);
       });
       void extension.start();`
      );

      const child = spawn(process.execPath, [join(tempDir, "index.mjs")], {
        stdio: ["pipe", "pipe", "pipe"],
      });
      const stdoutInterface = readline.createInterface({ input: child.stdout });
      const stdout = new BufferedMessageReader(stdoutInterface);
      const stderr = readline.createInterface({ input: child.stderr });
      const stderrLines: string[] = [];
      stderr.on("line", line => {
        stderrLines.push(line);
      });

      try {
        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: 1,
            method: "initialize",
            params: {
              protocol_version: "1",
              supported_protocol_versions: ["1"],
              agh_version: "0.5.0",
              session_nonce: "integration-nonce",
              extension: { name: "integration-ext", version: "0.1.0", source_tier: "user" },
              capabilities: {
                provides: ["memory.backend"],
                granted_actions: ["sessions/list"],
                granted_security: ["memory.read", "memory.write", "session.read"],
                granted_resource_kinds: [],
                granted_resource_scopes: [],
              },
              methods: {
                daemon_requests: ["health_check", "shutdown"],
                extension_services: ["memory/store", "memory/recall", "memory/forget"],
              },
              runtime: {
                health_check_interval_ms: 30000,
                health_check_timeout_ms: 5000,
                shutdown_timeout_ms: 10000,
                default_hook_timeout_ms: 5000,
              },
            },
          })}\n`
        );

        const diagnostics = { child, label: "initialize response", stderrLines };
        const initializeResponse = await stdout.next(diagnostics);
        expect(initializeResponse).toMatchObject({
          id: 1,
          result: expect.objectContaining({
            protocol_version: "1",
          }),
        });

        const sessionsListRequest = await stdout.next({
          ...diagnostics,
          label: "sessions/list request",
        });
        expect(sessionsListRequest).toMatchObject({
          method: "sessions/list",
        });
        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: sessionsListRequest.id,
            result: [
              {
                id: "sess-1",
                agent: "claude",
                state: "active",
                created_at: "2026-04-10T12:00:00.000Z",
              },
            ],
          })}\n`
        );

        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: 2,
            method: "memory/store",
            params: {
              key: "alpha",
              content: "remember this",
            },
          })}\n`
        );
        const storeResponse = await stdout.next({
          ...diagnostics,
          label: "memory/store response",
        });
        expect(storeResponse).toMatchObject({
          id: 2,
          result: { stored: "alpha" },
        });

        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: 3,
            method: "health_check",
            params: {},
          })}\n`
        );
        const healthResponse = await stdout.next({
          ...diagnostics,
          label: "health_check response",
        });
        expect(healthResponse).toMatchObject({
          id: 3,
          result: { healthy: true },
        });

        await waitForStderrLine(
          stderr,
          { ...diagnostics, label: "ready callback stderr" },
          /ready sessions.*1/
        );
      } finally {
        await terminateChild(child, [stdoutInterface, stderr]);
      }
    },
    integrationTimeoutMs
  );

  it(
    "serves extension.tool descriptors and calls over real stdio",
    async () => {
      const sdkEntry = resolve(packageDir, "dist/esm/index.js");
      const tempDir = await mkdtemp(join(tmpdir(), "agh-sdk-tool-integration-"));
      tempDirs.push(tempDir);
      await writeFile(
        join(tempDir, "index.mjs"),
        `import { Extension } from ${JSON.stringify(sdkEntry)};
       const extension = new Extension({ name: "tool-ext", version: "0.1.0" });
       extension.tool("search", {
         readOnly: true,
         inputSchema: {
           type: "object",
           required: ["query"],
           properties: { query: { type: "string" } }
         }
       }, async ({ input }) => ({
         content: [{ type: "text", text: "result " + input.query }],
         truncated: false,
         bytes: 0,
         duration_ms: 0
       }));
       void extension.start();`
      );

      const child = spawn(process.execPath, [join(tempDir, "index.mjs")], {
        stdio: ["pipe", "pipe", "pipe"],
      });
      const stdoutInterface = readline.createInterface({ input: child.stdout });
      const stdout = new BufferedMessageReader(stdoutInterface);
      const stderr = readline.createInterface({ input: child.stderr });
      const stderrLines: string[] = [];
      stderr.on("line", line => {
        stderrLines.push(line);
      });

      try {
        const diagnostics = { child, label: "tool initialize response", stderrLines };
        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: 1,
            method: "initialize",
            params: {
              protocol_version: "1",
              supported_protocol_versions: ["1"],
              agh_version: "0.5.0",
              session_nonce: "tool-nonce",
              extension: { name: "tool-ext", version: "0.1.0", source_tier: "user" },
              capabilities: {
                provides: ["tool.provider"],
                granted_actions: [],
                granted_security: [],
                granted_resource_kinds: [],
                granted_resource_scopes: [],
              },
              methods: {
                daemon_requests: ["health_check", "shutdown"],
                extension_services: ["provide_tools", "tools/call"],
              },
              runtime: {
                health_check_interval_ms: 30000,
                health_check_timeout_ms: 5000,
                shutdown_timeout_ms: 10000,
                default_hook_timeout_ms: 5000,
              },
            },
          })}\n`
        );

        await expect(stdout.next(diagnostics)).resolves.toMatchObject({
          id: 1,
          result: {
            accepted_capabilities: { provides: ["tool.provider"] },
            implemented_methods: expect.arrayContaining(["provide_tools", "tools/call"]),
          },
        });

        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: 2,
            method: "provide_tools",
            params: {},
          })}\n`
        );
        await expect(
          stdout.next({ ...diagnostics, label: "provide_tools response" })
        ).resolves.toMatchObject({
          id: 2,
          result: {
            tools: [
              {
                id: "ext__tool_ext__search",
                handler: "search",
                read_only: true,
                risk: "read",
              },
            ],
          },
        });

        child.stdin.write(
          `${JSON.stringify({
            jsonrpc: "2.0",
            id: 3,
            method: "tools/call",
            params: {
              tool_id: "ext__tool_ext__search",
              handler: "search",
              session_id: "session-1",
              input: { query: "alpha" },
            },
          })}\n`
        );
        await expect(
          stdout.next({ ...diagnostics, label: "tools/call response" })
        ).resolves.toMatchObject({
          id: 3,
          result: {
            result: {
              content: [{ type: "text", text: "result alpha" }],
              truncated: false,
              bytes: 0,
              duration_ms: 0,
            },
          },
        });
      } finally {
        await terminateChild(child, [stdoutInterface, stderr]);
      }
    },
    integrationTimeoutMs
  );
});
