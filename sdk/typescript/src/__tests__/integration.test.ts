import { execFileSync } from "node:child_process";
import { readdir, stat } from "node:fs/promises";
import { join, resolve } from "node:path";

import { beforeAll, describe, expect, it } from "vitest";

import { publicProvideFixtures, validateProvideConformance } from "../testing/conformance.js";

import { withExtensionProcess, type RpcMessage } from "./stdio-integration-harness.js";

const sourceDir = resolve(__dirname, "..");
const packageDir = resolve(sourceDir, "..");
const workspaceRoot = resolve(packageDir, "../..");
const tscBin = join(workspaceRoot, "node_modules/.bin/tsc");
const buildTimeoutMs = 120_000;
// Leave room for all per-message deadlines and subprocess teardown.
const exchangeTimeoutMs = 35_000;
const sdkEntry = resolve(packageDir, "dist/esm/index.js");

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

describe("SDK integration", () => {
  beforeAll(async () => {
    await ensureBuiltSDK();
  }, buildTimeoutMs);

  it("runs generated conformance fixtures for every public provide", () => {
    const fixtures = publicProvideFixtures();
    const provides = fixtures.map(fixture => fixture.provide);
    expect(provides).toEqual([
      "forge.provider",
      "heartbeat.source",
      "loop.watch_source",
      "model.source",
      "tool.provider",
      "view.provider",
    ]);

    for (const fixture of fixtures) {
      expect(() =>
        validateProvideConformance(fixture.provide, fixture.requiredMethods)
      ).not.toThrow();
      expect(fixture.requiredMethods.length).toBeGreaterThan(0);
      expect(() =>
        validateProvideConformance(fixture.provide, fixture.requiredMethods.slice(1))
      ).toThrow(fixture.requiredMethods[0]);
    }
    expect(() => validateProvideConformance("unknown.provider", ["unknown/method"])).toThrow(
      "no public conformance fixture"
    );
  });

  describe("model source extension", () => {
    let exchange: {
      initializeResponse: RpcMessage;
      sessionsListRequest: RpcMessage;
      modelsResponse: RpcMessage;
      healthResponse: RpcMessage;
    };

    beforeAll(async () => {
      exchange = await withExtensionProcess(
        `import { Extension } from ${JSON.stringify(sdkEntry)};
       const extension = new Extension(
         {
           name: "integration-ext",
           version: "0.1.0",
           capabilities: { provides: ["model.source"] },
           permissions: { requires: ["sessions/list"] }
         }
       );
       extension.handle("models/list", async (_ctx, params) => ({ models: [params.source] }));
       extension.handle("health_check", async () => ({ healthy: true, message: "", details: {} }));
       extension.onReady(async (host) => {
         const sessions = await host.sessions.list();
         console.error("ready sessions", sessions.length);
       });
       void extension.start();`,
        async ({ send, read, waitForStderr }) => {
          send({
            id: 1,
            method: "initialize",
            params: {
              protocol_version: "1",
              supported_protocol_versions: ["1"],
              compozy_version: "0.5.0",
              session_nonce: "integration-nonce",
              extension: { name: "integration-ext", version: "0.1.0", source_tier: "user" },
              capabilities: {
                provides: ["model.source"],
                granted_permissions: ["sessions/list"],
                granted_resource_kinds: [],
                granted_resource_scopes: [],
              },
              methods: {
                daemon_requests: ["health_check", "shutdown"],
                extension_services: ["models/list"],
              },
              runtime: {
                health_check_interval_ms: 30000,
                health_check_timeout_ms: 5000,
                shutdown_timeout_ms: 10000,
                default_hook_timeout_ms: 5000,
              },
            },
          });

          const initializeResponse = await read("initialize response");

          const sessionsListRequest = await read("sessions/list request");
          send({
            id: sessionsListRequest.id,
            result: [
              {
                id: "sess-1",
                agent: "claude",
                state: "active",
                created_at: "2026-04-10T12:00:00.000Z",
              },
            ],
          });

          send({
            id: 2,
            method: "models/list",
            params: {
              source: "alpha",
            },
          });
          const modelsResponse = await read("models/list response");

          send({
            id: 3,
            method: "health_check",
            params: {},
          });
          const healthResponse = await read("health_check response");

          await waitForStderr("ready callback stderr", /ready sessions.*1/);
          return { initializeResponse, sessionsListRequest, modelsResponse, healthResponse };
        }
      );
    }, exchangeTimeoutMs);

    it("builds an SDK-based extension and serves real JSON-RPC over stdio", () => {
      const { initializeResponse, sessionsListRequest, modelsResponse, healthResponse } = exchange;
      expect(initializeResponse).toMatchObject({
        id: 1,
        result: expect.objectContaining({
          protocol_version: "1",
        }),
      });
      expect(sessionsListRequest).toMatchObject({
        method: "sessions/list",
      });
      expect(modelsResponse).toMatchObject({
        id: 2,
        result: { models: ["alpha"] },
      });
      expect(healthResponse).toMatchObject({
        id: 3,
        result: { healthy: true },
      });
    });
  });

  describe("tool provider extension", () => {
    let exchange: {
      initializeResponse: RpcMessage;
      provideToolsResponse: RpcMessage;
      toolCallResponse: RpcMessage;
    };

    beforeAll(async () => {
      exchange = await withExtensionProcess(
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
       void extension.start();`,
        async ({ send, read }) => {
          send({
            id: 1,
            method: "initialize",
            params: {
              protocol_version: "1",
              supported_protocol_versions: ["1"],
              compozy_version: "0.5.0",
              session_nonce: "tool-nonce",
              extension: { name: "tool-ext", version: "0.1.0", source_tier: "user" },
              capabilities: {
                provides: ["tool.provider"],
                granted_permissions: [],
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
          });

          const initializeResponse = await read("tool initialize response");

          send({
            id: 2,
            method: "provide_tools",
            params: {},
          });
          const provideToolsResponse = await read("provide_tools response");

          send({
            id: 3,
            method: "tools/call",
            params: {
              tool_id: "ext__tool_ext__search",
              handler: "search",
              session_id: "session-1",
              input: { query: "alpha" },
            },
          });
          const toolCallResponse = await read("tools/call response");
          return { initializeResponse, provideToolsResponse, toolCallResponse };
        }
      );
    }, exchangeTimeoutMs);

    it("serves extension.tool descriptors and calls over real stdio", () => {
      const { initializeResponse, provideToolsResponse, toolCallResponse } = exchange;
      expect(initializeResponse).toMatchObject({
        id: 1,
        result: {
          accepted_capabilities: { provides: ["tool.provider"] },
          implemented_methods: expect.arrayContaining(["provide_tools", "tools/call"]),
        },
      });
      expect(provideToolsResponse).toMatchObject({
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
      expect(toolCallResponse).toMatchObject({
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
    });
  });
});
