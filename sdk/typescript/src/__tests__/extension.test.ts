import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it, vi } from "vitest";

import { Extension } from "../extension.js";
import {
  CONNECTIVITY_ESTABLISH_METHOD,
  CONNECTIVITY_STATUS_METHOD,
  CONNECTIVITY_TEARDOWN_METHOD,
  registerConnectivityProvider,
  type ConnectivityProviderHandlers,
} from "../connectivity-provider.js";
import {
  FORGE_CAPABILITIES_METHOD,
  FORGE_PR_CREATE_METHOD,
  FORGE_STATUS_METHOD,
  registerForgeProvider,
  type ForgeProviderHandlers,
} from "../forge-provider.js";
import {
  CapabilityDeniedError,
  InternalError,
  InvalidParamsError,
  MethodNotFoundError,
  NotInitializedError,
  ShutdownInProgressError,
  ToolExecutionError,
} from "../errors.js";
import { canonicalJSON, schemaDigest } from "../schema-digest.js";
import { TestHarness } from "../testing/harness.js";
import { createMockTransportPair, MockTransport } from "../testing/mock-transport.js";
import type { TransportHandler } from "../transport.js";
import type { DescribePayload, InitializeRequest, JSONValue } from "../types.js";

interface SchemaDigestFixture {
  name: string;
  schema: JSONValue;
  canonical: string;
  sha256: string;
}

class FailingProvideTransport extends MockTransport {
  public readonly connectivityHandlers = new Set<string>();

  public override handle(method: string, handler: TransportHandler): void {
    super.handle(method, handler);
    if (!method.startsWith("connectivity/")) {
      return;
    }
    this.connectivityHandlers.add(method);
    if (method === CONNECTIVITY_STATUS_METHOD) {
      throw new Error("connectivity transport bind failed");
    }
  }

  public override unhandle(method: string): void {
    super.unhandle(method);
    this.connectivityHandlers.delete(method);
  }
}

function initializeFor(extension: Extension): InitializeRequest {
  const methods = extension.getImplementedMethods();
  return {
    protocol_version: "1",
    supported_protocol_versions: ["1"],
    compozy_version: "0.5.0",
    session_nonce: "session-nonce-test",
    extension: {
      name: extension.definition.name,
      version: extension.definition.version,
      source_tier: "user",
    },
    capabilities: {
      provides: extension.definition.capabilities?.provides ?? [],
      granted_permissions: extension.definition.permissions?.requires ?? [],
      granted_resource_kinds: [],
      granted_resource_scopes: [],
    },
    methods: {
      daemon_requests: methods.filter(method =>
        ["execute_hook", "health_check", "shutdown"].includes(method)
      ),
      extension_services: methods.filter(
        method => !["execute_hook", "health_check", "shutdown"].includes(method)
      ),
    },
    runtime: {
      health_check_interval_ms: 30_000,
      health_check_timeout_ms: 5_000,
      shutdown_timeout_ms: 10_000,
      default_hook_timeout_ms: 5_000,
    },
  };
}

describe("Extension", () => {
  it("start performs initialize handshake first", async () => {
    const pair = createMockTransportPair();
    const extension = new Extension(
      {
        name: "test-ext",
        version: "0.1.0",
      },
      { transport: pair.extension }
    );

    const startPromise = extension.start();

    await expect(pair.host.call("health_check", {})).rejects.toBeInstanceOf(NotInitializedError);
    await expect(pair.host.call("initialize", initializeFor(extension))).resolves.toMatchObject({
      protocol_version: "1",
    });
    await expect(startPromise).resolves.toBeDefined();
  });

  it("handle routes inbound requests to the correct handler", async () => {
    const harness = new TestHarness();
    const extension = new Extension({
      name: "router",
      version: "0.1.0",
    });

    extension.handle("memory/store", async (_ctx, params: { key: string }) => ({
      stored: params.key,
    }));
    await harness.loadExtension(extension);

    await expect(harness.call("memory/store", { key: "alpha" })).resolves.toEqual({
      stored: "alpha",
    });
  });

  it("returns method not found when no handler is registered", async () => {
    const harness = new TestHarness();
    const extension = new Extension({
      name: "missing",
      version: "0.1.0",
    });

    await harness.loadExtension(extension);
    await expect(harness.call("memory/store", { key: "x" })).rejects.toBeInstanceOf(
      MethodNotFoundError
    );
  });

  it("onReady fires after successful handshake", async () => {
    const ready = vi.fn(async () => {});
    const harness = new TestHarness();
    const extension = new Extension({
      name: "ready",
      version: "0.1.0",
    });

    extension.onReady(ready);
    await harness.loadExtension(extension);

    expect(ready).toHaveBeenCalledTimes(1);
  });

  it("rejects initialize when required grants are missing", async () => {
    const pair = createMockTransportPair();
    const extension = new Extension(
      {
        name: "denied",
        version: "0.1.0",
        permissions: { requires: ["sessions/list"] },
      },
      { transport: pair.extension }
    );

    void extension.start();
    await expect(
      pair.host.call("initialize", {
        ...initializeFor(extension),
        capabilities: {
          provides: [],
          granted_permissions: [],
          granted_resource_kinds: [],
          granted_resource_scopes: [],
        },
      })
    ).rejects.toBeInstanceOf(CapabilityDeniedError);
  });

  it("serves default health checks", async () => {
    const harness = new TestHarness();
    const extension = new Extension({
      name: "tools",
      version: "0.1.0",
    });

    await harness.loadExtension(extension, {
      daemonRequests: ["health_check", "shutdown"],
    });
    await expect(harness.call("health_check", {})).resolves.toMatchObject({ healthy: true });
  });

  it("negotiates bridge adapter services and exposes scoped runtime data", async () => {
    const ready = vi.fn();
    const harness = new TestHarness();
    const extension = new Extension({
      name: "bridge-adapter",
      version: "0.1.0",
      capabilities: { provides: ["bridge.adapter"] },
      permissions: { requires: ["bridges/instances/get"] },
    });

    extension.handle("bridges/deliver", async () => ({
      acknowledged: true,
    }));
    extension.handle("bridges/targets/snapshot", async () => ({ targets: [] }));
    extension.onReady((_host, session) => {
      ready(session.initializeRequest.runtime.bridge?.managed_instances?.[0]?.instance.id);
    });

    await harness.loadExtension(extension, {
      grantedPermissions: ["bridges/instances/get"],
      runtime: {
        bridge: {
          runtime_version: "1",
          purpose: "service",
          provider: "bridge-adapter",
          platform: "telegram",
          managed_instances: [
            {
              instance: {
                id: "chan-1",
                scope: "global",
                platform: "telegram",
                extension_name: "bridge-adapter",
                display_name: "Telegram",
                enabled: true,
                status: "ready",
                notification_suppress: false,
                routing_policy: {
                  include_peer: true,
                  include_thread: false,
                  include_group: false,
                },
                created_at: "2026-04-11T12:00:00.000Z",
                updated_at: "2026-04-11T12:00:00.000Z",
              },
              bound_secrets: [{ binding_name: "bot_token", kind: "bot_token", value: "secret" }],
            },
          ],
        },
      },
    });

    expect(harness.getLastInitializeRequest()).toMatchObject({
      session_nonce: "session-nonce-test",
      capabilities: {
        granted_resource_kinds: [],
        granted_resource_scopes: [],
      },
      methods: {
        extension_services: ["bridges/deliver", "bridges/targets/snapshot"],
      },
    });
    expect(ready).toHaveBeenCalledWith("chan-1");
  });

  it("captures session nonce and resource grants during initialize", async () => {
    const ready = vi.fn();
    const harness = new TestHarness();
    const extension = new Extension({
      name: "resource-ext",
      version: "0.1.0",
    });

    extension.onReady((_host, session) => {
      ready({
        session_nonce: session.initializeRequest.session_nonce,
        granted_resource_kinds: session.initializeRequest.capabilities.granted_resource_kinds,
        granted_resource_scopes: session.initializeRequest.capabilities.granted_resource_scopes,
      });
    });

    await harness.loadExtension(extension, {
      sessionNonce: "nonce-resource",
      grantedResourceKinds: ["tool", "skill"],
      grantedResourceScopes: ["workspace"],
    });

    expect(ready).toHaveBeenCalledWith({
      session_nonce: "nonce-resource",
      granted_resource_kinds: ["tool", "skill"],
      granted_resource_scopes: ["workspace"],
    });
  });

  it("rejects bridge.adapter initialize when bridges/deliver is not implemented", async () => {
    const pair = createMockTransportPair();
    const extension = new Extension(
      {
        name: "bridge-denied",
        version: "0.1.0",
        capabilities: { provides: ["bridge.adapter"] },
      },
      { transport: pair.extension }
    );

    void extension.start();
    await expect(pair.host.call("initialize", initializeFor(extension))).rejects.toMatchObject({
      data: {
        error: expect.stringContaining("bridge.adapter"),
      },
    });
  });

  it("rejects new work after shutdown begins", async () => {
    const harness = new TestHarness();
    const extension = new Extension({
      name: "shutdown",
      version: "0.1.0",
    });

    await harness.loadExtension(extension);
    await expect(harness.call("shutdown", { reason: "test", deadline_ms: 1000 })).resolves.toEqual({
      acknowledged: true,
    });
    await expect(harness.call("health_check", {})).rejects.toBeInstanceOf(ShutdownInProgressError);
  });

  it("registers executable tools and preserves trusted invocation context", async () => {
    const harness = new TestHarness();
    const clarify = vi.fn(async () => ({ text: "ship it", fallback: false }));
    harness.mockHostAPI("clarify/ask", clarify);
    const extension = new Extension({
      name: "linear",
      version: "0.1.0",
      permissions: { requires: ["clarify/ask"] },
    });

    extension.tool<{ query: string }>(
      "search",
      {
        readOnly: true,
        inputSchema: {
          required: ["query"],
          type: "object",
          properties: {
            query: { type: "string" },
          },
        },
      },
      async ({ input, trustedWorkspace, invocationId, askClarification }) => {
        expect(trustedWorkspace).toEqual({ id: "workspace-1", root: "/workspace/one" });
        expect(invocationId).toBe("invocation-1");
        const answer = await askClarification({ question: "Deploy?", choices: ["yes", "no"] });
        return {
          content: [{ type: "text", text: `found ${input.query}: ${answer.text}` }],
          truncated: false,
          bytes: 0,
          duration_ms: 0,
        };
      }
    );

    await harness.loadExtension(extension);

    expect(extension.definition.capabilities?.provides).toContain("tool.provider");
    expect(harness.getLastInitializeResponse()?.implemented_methods).toEqual(
      expect.arrayContaining(["provide_tools", "tools/call"])
    );

    await expect(harness.call("provide_tools", {})).resolves.toEqual({
      tools: [
        {
          id: "ext__linear__search",
          handler: "search",
          input_schema: {
            required: ["query"],
            type: "object",
            properties: {
              query: { type: "string" },
            },
          },
          input_schema_digest: "1dc63095e8672403bbe40fa26719d175e695c0167f6daad6b9655f6506491f01",
          read_only: true,
          requires_interaction: false,
          risk: "read",
          capabilities: [],
        },
      ],
    });

    await expect(
      harness.call("tools/call", {
        tool_id: "ext__linear__search",
        handler: "search",
        session_id: "session-1",
        invocation_id: "invocation-1",
        trusted_workspace: { id: "workspace-1", root: "/workspace/one" },
        input: { query: "alpha" },
      })
    ).resolves.toEqual({
      result: {
        content: [{ type: "text", text: "found alpha: ship it" }],
        truncated: false,
        bytes: 0,
        duration_ms: 0,
      },
    });
    expect(clarify).toHaveBeenCalledWith({
      invocation_id: "invocation-1",
      question: "Deploy?",
      choices: ["yes", "no"],
    });
  });

  it("prints a deterministic describe payload and exits without transport startup", async () => {
    const harness = new TestHarness();
    const extension = new Extension(
      {
        name: "describe-fixture",
        version: "0.1.0",
        description: "Describe fixture",
        subprocess: { command: "node", args: ["index.js"] },
        permissions: { requires: ["sessions/list"] },
        resources: {
          skills: [" skills/zeta ", "skills/alpha", "skills/zeta"],
          loops: [" loops/zeta ", "loops/alpha", "loops/zeta"],
          agents: [" agents/zeta ", "agents/alpha", "agents/zeta"],
          automation: [" automation/zeta ", "automation/alpha", "automation/zeta"],
          layouts: [" layouts/zeta ", "layouts/alpha", "layouts/zeta"],
        },
        supported_hook_events: ["prompt.post_assemble"],
      },
      { describeProcess: harness.captureDescribeProcess() }
    );
    extension.tool<{ query: string }>(
      "search",
      {
        description: "Search extension data",
        readOnly: true,
        requiresInteraction: true,
        inputSchema: {
          type: "object",
          required: ["query"],
          properties: { query: { type: "string" } },
        },
        command: {
          verb: "review/search",
          summary: "Search reviews",
          flags: { query: "query" },
        },
      },
      async () => ({ content: [], truncated: false, bytes: 0, duration_ms: 0 })
    );
    extension.commandGroup("review", "Review commands");
    extension.watchSource("reviews", {}, async () => ({
      ready: false,
      event_key: "reviews:describe",
    }));

    await expect(extension.start()).resolves.toBeDefined();
    const result = harness.getDescribeResult();
    expect(result.exitCode).toBe(0);
    expect(result.stdout.trim().split("\n")).toHaveLength(1);
    const payload = JSON.parse(result.stdout) as DescribePayload;
    expect(payload.resources).toEqual({
      skills: ["skills/alpha", "skills/zeta"],
      loops: ["loops/alpha", "loops/zeta"],
      agents: ["agents/alpha", "agents/zeta"],
      automation: ["automation/alpha", "automation/zeta"],
      layouts: ["layouts/alpha", "layouts/zeta"],
    });
    expect(payload).toMatchObject({
      name: "describe-fixture",
      version: "0.1.0",
      description: "Describe fixture",
      provides: ["loop.watch_source", "tool.provider"],
      permissions: ["sessions/list"],
      subprocess: { command: "node", args: ["index.js"] },
      tools: [
        {
          id: "ext__describe_fixture__search",
          handler: "search",
          description: "Search extension data",
          input_schema: {
            type: "object",
            required: ["query"],
            properties: { query: { type: "string" } },
          },
          input_schema_digest: schemaDigest({
            type: "object",
            required: ["query"],
            properties: { query: { type: "string" } },
          }),
          requires_interaction: true,
          command: {
            verb: "review/search",
            summary: "Search reviews",
            flags: { query: "query" },
          },
        },
      ],
      command_groups: [{ path: "review", summary: "Review commands" }],
      hook_events: ["prompt.post_assemble"],
      watch_source_kinds: ["reviews"],
      sdk: {
        name: "@compozy/extension-sdk",
        protocol_version: "1",
        min_compozy_version: "0.3.0-beta.1",
      },
    });
  });

  it("rejects model source initialize when generated required methods are missing", async () => {
    const pair = createMockTransportPair();
    const extension = new Extension(
      {
        name: "incomplete-model-source",
        version: "0.1.0",
        capabilities: { provides: ["model.source"] },
      },
      { transport: pair.extension }
    );

    void extension.start();
    await expect(pair.host.call("initialize", initializeFor(extension))).rejects.toMatchObject({
      constructor: InternalError,
      data: { error: expect.stringContaining("models/list") },
    });
  });

  it("registers watch sources and serves watch/poll", async () => {
    const harness = new TestHarness();
    const extension = new Extension({
      name: "review-watcher",
      version: "0.1.0",
    });

    extension.watchSource<{ kind: string; query: string }>(
      "reviews",
      {},
      async ({ spec, expectedStateDigest }) => ({
        ready: spec.query === "open",
        event_key: "reviews:r1",
        state_digest: expectedStateDigest === "sha256:previous" ? "sha256:next" : "sha256:first",
        payload: { review: "r1" },
        settled_at: "2026-07-05T12:00:00.000Z",
      })
    );

    await harness.loadExtension(extension);

    expect(extension.definition.capabilities?.provides).toContain("loop.watch_source");
    expect(harness.getLastInitializeResponse()?.implemented_methods).toEqual(
      expect.arrayContaining(["watch/poll"])
    );
    expect(harness.getLastInitializeResponse()?.watch_source_kinds).toEqual(["reviews"]);

    await expect(
      harness.call("watch/poll", {
        spec: { kind: "reviews", query: "open" },
        expected_state_digest: "sha256:previous",
      })
    ).resolves.toEqual({
      ready: true,
      event_key: "reviews:r1",
      state_digest: "sha256:next",
      payload: { review: "r1" },
      settled_at: "2026-07-05T12:00:00.000Z",
    });
  });

  it("registers and serves a connectivity provider", async () => {
    const harness = new TestHarness();
    const extension = new Extension({ name: "connectivity", version: "0.1.0" });
    registerConnectivityProvider(extension, {
      establish: (_context, request) => ({
        tier: request.tier,
        endpoints: [{ url: "https://gateway.example.test", scheme: "https", stability: "stable" }],
        health: "healthy",
      }),
      status: (_context, request) => ({
        tier: request.tier,
        endpoints: [{ url: "https://gateway.example.test", scheme: "https", stability: "stable" }],
        health: "healthy",
      }),
      teardown: () => ({ stopped: true }),
    });
    await harness.loadExtension(extension);
    expect(extension.definition.capabilities?.provides).toContain("connectivity.provider");
    expect(harness.getLastInitializeResponse()?.implemented_methods).toEqual(
      expect.arrayContaining([
        "connectivity/establish",
        "connectivity/status",
        "connectivity/teardown",
      ])
    );
    await expect(
      harness.call("connectivity/establish", {
        tier: "private",
        forward_target: "127.0.0.1:43210",
        challenge_path: "/.well-known/compozy/gateway-challenge/typescript",
        deadline: "2026-08-07T12:00:00Z",
      })
    ).resolves.toMatchObject({ tier: "private", health: "healthy" });
    await expect(harness.call("connectivity/status", { tier: "private" })).resolves.toMatchObject({
      tier: "private",
      health: "healthy",
    });
    await expect(
      harness.call("connectivity/teardown", {
        tier: "private",
        deadline: "2026-08-07T12:00:00Z",
      })
    ).resolves.toEqual({ stopped: true });
  });

  it("registers and serves a forge provider", async () => {
    const harness = new TestHarness();
    const extension = new Extension({ name: "forge", version: "0.1.0" });
    registerForgeProvider(extension, forgeHandlers());
    await harness.loadExtension(extension);

    expect(extension.definition.capabilities?.provides).toContain("forge.provider");
    expect(harness.getLastInitializeResponse()?.implemented_methods).toEqual(
      expect.arrayContaining([
        FORGE_CAPABILITIES_METHOD,
        FORGE_STATUS_METHOD,
        FORGE_PR_CREATE_METHOD,
      ])
    );
    await expect(
      harness.call(FORGE_CAPABILITIES_METHOD, {
        remote_urls: ["https://github.com/acme/repo.git"],
      })
    ).resolves.toMatchObject({ served: true, provider: "test" });
    await expect(
      harness.call(FORGE_STATUS_METHOD, {
        remote_urls: ["https://github.com/acme/repo.git"],
        branch: "feature/exit",
      })
    ).resolves.toMatchObject({ provider: "test", pr_number: 7 });
    await expect(
      harness.call(FORGE_PR_CREATE_METHOD, {
        remote_urls: ["https://github.com/acme/repo.git"],
        head: "feature/exit",
        base: "main",
        title: "Exit",
      })
    ).resolves.toMatchObject({ status: "created", number: 8 });
  });

  it("keeps forge provider registration atomic across collisions", () => {
    const extension = new Extension({ name: "forge-collision", version: "0.1.0" });
    extension.handle(FORGE_STATUS_METHOD, () => ({ provider: "existing" }));

    expect(() => registerForgeProvider(extension, forgeHandlers())).toThrow(
      "forge/status is already registered"
    );
    expect(extension.definition.capabilities?.provides ?? []).not.toContain("forge.provider");
    expect(() => extension.handle(FORGE_CAPABILITIES_METHOD, () => ({}))).not.toThrow();
    expect(() => extension.handle(FORGE_PR_CREATE_METHOD, () => ({}))).not.toThrow();
  });

  it("rejects malformed connectivity requests before calling provider handlers", async () => {
    const harness = new TestHarness();
    const establish = vi.fn<ConnectivityProviderHandlers["establish"]>(() => ({
      tier: "private",
      endpoints: [],
      health: "healthy",
    }));
    const status = vi.fn<ConnectivityProviderHandlers["status"]>(() => ({
      tier: "private",
      endpoints: [],
      health: "healthy",
    }));
    const teardown = vi.fn<ConnectivityProviderHandlers["teardown"]>(() => ({ stopped: true }));
    const extension = new Extension({ name: "connectivity-validation", version: "0.1.0" });
    registerConnectivityProvider(extension, { establish, status, teardown });
    await harness.loadExtension(extension);

    await expect(
      harness.call(CONNECTIVITY_ESTABLISH_METHOD, {
        tier: "private",
        forward_target: "127.0.0.1:43210",
        challenge_path: "/challenge",
      })
    ).rejects.toBeInstanceOf(InvalidParamsError);
    await expect(harness.call(CONNECTIVITY_STATUS_METHOD, { tier: 42 })).rejects.toBeInstanceOf(
      InvalidParamsError
    );
    await expect(harness.call(CONNECTIVITY_TEARDOWN_METHOD, null)).rejects.toBeInstanceOf(
      InvalidParamsError
    );

    expect(establish).not.toHaveBeenCalled();
    expect(status).not.toHaveBeenCalled();
    expect(teardown).not.toHaveBeenCalled();
  });

  it("keeps connectivity provider registration atomic across collisions", () => {
    const extension = new Extension({ name: "connectivity-collision", version: "0.1.0" });
    extension.handle(CONNECTIVITY_STATUS_METHOD, () => ({ tier: "private", health: "down" }));

    expect(() => registerConnectivityProvider(extension, connectivityHandlers())).toThrow(
      "connectivity/status is already registered"
    );
    expect(extension.definition.capabilities?.provides ?? []).not.toContain(
      "connectivity.provider"
    );
    expect(() => extension.handle(CONNECTIVITY_ESTABLISH_METHOD, () => ({}))).not.toThrow();
    expect(() => extension.handle(CONNECTIVITY_TEARDOWN_METHOD, () => ({}))).not.toThrow();
  });

  it("rolls back transport handlers when connectivity registration fails", () => {
    const transport = new FailingProvideTransport();
    const extension = new Extension(
      { name: "connectivity-bind-failure", version: "0.1.0" },
      { transport }
    );

    expect(() => registerConnectivityProvider(extension, connectivityHandlers())).toThrow(
      "connectivity transport bind failed"
    );
    expect(transport.connectivityHandlers.size).toBe(0);
    expect(extension.definition.capabilities?.provides ?? []).not.toContain(
      "connectivity.provider"
    );
    expect(extension.getImplementedMethods()).not.toEqual(
      expect.arrayContaining([
        CONNECTIVITY_ESTABLISH_METHOD,
        CONNECTIVITY_STATUS_METHOD,
        CONNECTIVITY_TEARDOWN_METHOD,
      ])
    );
  });

  it("rejects duplicate and every post-start registration surface", async () => {
    const duplicate = new Extension({ name: "connectivity-duplicate", version: "0.1.0" });
    registerConnectivityProvider(duplicate, connectivityHandlers());
    expect(() => registerConnectivityProvider(duplicate, connectivityHandlers())).toThrow(
      "connectivity/establish is already registered"
    );

    const pair = createMockTransportPair();
    const late = new Extension(
      { name: "connectivity-late", version: "0.1.0" },
      { transport: pair.extension }
    );
    const started = late.start();
    expect(() => registerConnectivityProvider(late, connectivityHandlers())).toThrow(
      "extension registration is closed after start"
    );
    expect(() =>
      late.tool("late-tool", { inputSchema: { type: "object" } }, async () => ({
        content: [],
        truncated: false,
        bytes: 0,
        duration_ms: 0,
      }))
    ).toThrow("extension registration is closed after start");
    expect(() =>
      late.watchSource("late-watch", {}, async () => ({
        ready: false,
        event_key: "late-watch",
      }))
    ).toThrow("extension registration is closed after start");
    expect(() => late.commandGroup("late", "Late commands")).toThrow(
      "extension registration is closed after start"
    );
    await pair.host.call("initialize", initializeFor(late));
    await expect(started).resolves.toBeDefined();
  });

  it("redacts sensitive tool input from handler errors", async () => {
    const harness = new TestHarness();
    const extension = new Extension({
      name: "secrets",
      version: "0.1.0",
    });

    extension.tool<{ token: string }>(
      "lookup",
      {
        readOnly: true,
        inputSchema: {
          type: "object",
          required: ["token"],
          properties: {
            token: { type: "string" },
          },
        },
        sensitiveInputFields: ["token"],
      },
      async ({ input }) => {
        throw new Error(`backend rejected ${input.token}`);
      }
    );

    await harness.loadExtension(extension);

    await expect(
      harness.call("tools/call", {
        tool_id: "ext__secrets__lookup",
        handler: "lookup",
        input: { token: "secret-token" },
      })
    ).rejects.toMatchObject({
      constructor: ToolExecutionError,
      data: {
        tool_id: "ext__secrets__lookup",
        handler: "lookup",
        error: "backend rejected [REDACTED]",
        input: { token: "[REDACTED]" },
        sensitive_input_fields: ["token"],
      },
    });
  });

  it("matches daemon schema digest fixtures", () => {
    const fixturePath = join(__dirname, "../../test-fixtures/digest/cases.json");
    const fixtures = JSON.parse(readFileSync(fixturePath, "utf8")) as SchemaDigestFixture[];

    expect(fixtures.length).toBeGreaterThan(0);
    for (const fixture of fixtures) {
      expect(canonicalJSON(fixture.schema)).toBe(fixture.canonical);
      expect(schemaDigest(fixture.schema)).toBe(fixture.sha256);
    }
  });
});

function connectivityHandlers(): ConnectivityProviderHandlers {
  return {
    establish: (_context, request) => ({
      tier: request.tier,
      endpoints: [{ url: "https://gateway.example.test", scheme: "https", stability: "stable" }],
      health: "healthy",
    }),
    status: (_context, request) => ({
      tier: request.tier,
      endpoints: [{ url: "https://gateway.example.test", scheme: "https", stability: "stable" }],
      health: "healthy",
    }),
    teardown: () => ({ stopped: true }),
  };
}

function forgeHandlers(): ForgeProviderHandlers {
  return {
    capabilities: () => ({
      served: true,
      available: true,
      provider: "test",
      served_remote: "https://github.com/acme/repo",
    }),
    status: () => ({
      provider: "test",
      pr_number: 7,
      pr_state: "open",
      fetched_at: "2026-08-12T12:00:00Z",
    }),
    createPR: () => ({
      status: "created",
      number: 8,
      url: "https://github.com/acme/repo/pull/8",
    }),
  };
}
