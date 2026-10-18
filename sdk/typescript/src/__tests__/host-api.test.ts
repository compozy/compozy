import { describe, expect, it } from "vitest";

import {
  CapabilityDeniedError,
  InvalidParamsError,
  NotInitializedError,
  RateLimitedError,
  RPCError,
} from "../errors.js";
import { HostAPI } from "../host-api.js";
import { createMockTransportPair } from "../testing/mock-transport.js";

describe("HostAPI", () => {
  it("sessions.create sends correct JSON-RPC request", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("sessions/create", async params => {
      expect(params).toEqual({
        agent: "coder",
        workspace: "workspace-1",
      });
      return { session_id: "sess-1" };
    });

    await expect(
      host.sessions.create({ agent: "coder", workspace: "workspace-1" })
    ).resolves.toEqual({ session_id: "sess-1" });

    expect(pair.extension.requests[0]).toMatchObject({
      method: "sessions/create",
      params: {
        agent: "coder",
        workspace: "workspace-1",
      },
    });
  });

  it("sessions.list parses response array", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("sessions/list", async () => [
      {
        id: "sess-1",
        name: "debug",
        agent: "claude",
        runtime: { status: "unbound" },
        state: "active",
        created_at: "2026-04-10T12:00:00.000Z",
      },
    ]);

    await expect(host.sessions.list()).resolves.toEqual([
      {
        id: "sess-1",
        name: "debug",
        agent: "claude",
        runtime: { status: "unbound" },
        state: "active",
        created_at: "2026-04-10T12:00:00.000Z",
      },
    ]);
  });

  it("authored context helpers route through managed Host API methods", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });
    const soulPayload = {
      agent_name: "coder",
      enabled: true,
      present: true,
      active: true,
      valid: true,
      validation_status: "valid",
      digest: "sha256:soul",
      frontmatter: {},
      limits: { max_body_bytes: 65536 },
      config_provenance: {
        digest: "sha256:cfg",
        enabled: true,
        max_body_bytes: 65536,
        context_projection_bytes: 2048,
      },
    };
    const heartbeatPolicy = {
      agent_name: "coder",
      enabled: true,
      present: true,
      active: true,
      valid: true,
      validation_status: "valid",
      digest: "sha256:heartbeat",
      schema_version: 1,
      frontmatter: { version: 1, enabled: true, preferences: {}, context: {} },
      preferences: { min_interval: "5m", context: {} },
      config_provenance: {
        digest: "sha256:heartbeat-cfg",
        subset: {
          enabled: true,
          max_body_bytes: 65536,
          context_projection_bytes: 2048,
          min_interval: "5m",
          default_interval: "15m",
          wake_cooldown: "5m",
          max_wakes_per_cycle: 1,
          active_session_only: true,
          allow_active_hours_preferences: true,
          wake_event_retention: "24h",
          session_health_stale_after: "5m",
          session_health_hook_min_interval: "1m",
        },
      },
      prompt: {
        active: true,
        digest: "sha256:heartbeat",
        preferences: { min_interval: "5m", context: {} },
        truncated: false,
        max_bytes: 2048,
        max_body_bytes: 65536,
        context: {},
      },
      limits: { max_body_bytes: 65536, context_projection_bytes: 2048 },
    };

    pair.host.handle("agents/soul/get", async params => {
      expect(params).toEqual({ workspace_id: "ws-1", agent_name: "coder" });
      return soulPayload;
    });
    pair.host.handle("agents/soul/put", async params => {
      expect(params).toMatchObject({
        workspace_id: "ws-1",
        agent_name: "coder",
        expected_digest: "sha256:old",
      });
      return { soul: soulPayload, revision: { id: "rev-1", agent_name: "coder" } };
    });
    pair.host.handle("sessions/soul/refresh", async params => {
      expect(params).toEqual({
        workspace_id: "ws-1",
        session_id: "sess-1",
        expected_digest: "sha256:soul",
      });
      return soulPayload;
    });
    pair.host.handle("sessions/health/get", async params => {
      expect(params).toEqual({ workspace_id: "ws-1", session_id: "sess-1" });
      return {
        health: {
          session_id: "sess-1",
          workspace_id: "ws-1",
          agent_name: "coder",
          state: "idle",
          health: "healthy",
          active_prompt: false,
          attachable: true,
          eligible_for_wake: true,
          updated_at: "2026-04-10T12:00:00.000Z",
        },
      };
    });
    pair.host.handle("agents/heartbeat/status", async params => {
      expect(params).toEqual({
        workspace_id: "ws-1",
        agent_name: "coder",
        session_id: "sess-1",
        include_session_health: true,
        include_recent_wake_events: true,
      });
      return {
        agent_name: "coder",
        enabled: true,
        present: true,
        active: true,
        valid: true,
        validation_status: "valid",
        preferences: { min_interval: "5m", context: {} },
      };
    });
    pair.host.handle("agents/heartbeat/wake", async params => {
      expect(params).toEqual({
        workspace_id: "ws-1",
        agent_name: "coder",
        session_id: "sess-1",
        source: "manual",
      });
      return { decision: { result: "sent", reason: "wake_sent" } };
    });
    pair.host.handle("agents/heartbeat/get", async params => {
      expect(params).toEqual({ workspace_id: "ws-1", agent_name: "coder" });
      return heartbeatPolicy;
    });

    await expect(host.soul.get({ workspace_id: "ws-1", agent_name: "coder" })).resolves.toEqual(
      soulPayload
    );
    await expect(
      host.soul.put({
        workspace_id: "ws-1",
        agent_name: "coder",
        body: "---\nrole: helper\n---\nBody.",
        expected_digest: "sha256:old",
      })
    ).resolves.toMatchObject({ soul: soulPayload });
    await expect(
      host.sessions.refreshSoul({
        workspace_id: "ws-1",
        session_id: "sess-1",
        expected_digest: "sha256:soul",
      })
    ).resolves.toEqual(soulPayload);
    await expect(
      host.sessions.health({ workspace_id: "ws-1", session_id: "sess-1" })
    ).resolves.toMatchObject({
      health: { eligible_for_wake: true },
    });
    await expect(
      host.heartbeat.status({
        workspace_id: "ws-1",
        agent_name: "coder",
        session_id: "sess-1",
        include_session_health: true,
        include_recent_wake_events: true,
      })
    ).resolves.toMatchObject({ agent_name: "coder", validation_status: "valid" });
    await expect(
      host.heartbeat.wake({
        workspace_id: "ws-1",
        agent_name: "coder",
        session_id: "sess-1",
        source: "manual",
      })
    ).resolves.toMatchObject({ decision: { result: "sent" } });
    await expect(host.heartbeat.get({ workspace_id: "ws-1", agent_name: "coder" })).resolves.toBe(
      heartbeatPolicy
    );
  });

  it("logs.list supports since parameter", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("logs/list", async params => {
      expect(params).toEqual({
        workspace_id: "ws-1",
        since: "2026-04-10T12:00:00.000Z",
        limit: 5,
      });
      return [
        {
          type: "message.end",
          timestamp: "2026-04-10T12:01:00.000Z",
          data: { summary: "done" },
        },
      ];
    });

    await expect(
      host.logs.list({ workspace_id: "ws-1", since: "2026-04-10T12:00:00.000Z", limit: 5 })
    ).resolves.toHaveLength(1);
  });

  it("supports the remaining host api methods", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("sessions/prompt", async params => {
      expect(params).toEqual({
        workspace_id: "ws-1",
        session_id: "sess-1",
        message_id: "message-1",
        idempotency_key: "idempotency-1",
        message: "hello",
        runtime: {
          provider: "codex",
          model: "gpt-5.6",
          reasoning_effort: "high",
          speed: "fast",
        },
      });
      return {
        status: "accepted",
        turn_id: "turn-1",
        message_id: "message-1",
        idempotency_key: "idempotency-1",
        replayed: false,
      };
    });
    pair.host.handle("sessions/stop", async params => {
      expect(params).toEqual({ workspace_id: "ws-1", session_id: "sess-1" });
      return {};
    });
    pair.host.handle("sessions/archive", async params => {
      expect(params).toEqual({ workspace_id: "ws-1", session_id: "sess-1" });
      return {
        session_id: "sess-1",
        agent: "claude",
        runtime: { status: "unbound" },
        state: "stopped",
        archived_at: "2026-04-10T12:02:00.000Z",
        created_at: "2026-04-10T12:00:00.000Z",
        updated_at: "2026-04-10T12:02:00.000Z",
      };
    });
    pair.host.handle("sessions/unarchive", async params => {
      expect(params).toEqual({ workspace_id: "ws-1", session_id: "sess-1" });
      return {
        session_id: "sess-1",
        agent: "claude",
        runtime: { status: "unbound" },
        state: "stopped",
        archived_at: null,
        created_at: "2026-04-10T12:00:00.000Z",
        updated_at: "2026-04-10T12:03:00.000Z",
      };
    });
    pair.host.handle("sessions/status", async () => ({
      session_id: "sess-1",
      agent: "claude",
      runtime: { status: "ready", effective: { provider: "codex", model: "gpt-5.6" } },
      state: "active",
      created_at: "2026-04-10T12:00:00.000Z",
      updated_at: "2026-04-10T12:01:00.000Z",
    }));
    pair.host.handle("observe/health", async () => ({
      status: "ok",
      uptime_seconds: 1,
      active_sessions: 1,
      active_agents: 1,
      global_db_size_bytes: 1,
      session_db_size_bytes: 1,
      version: "0.5.0",
    }));
    pair.host.handle("skills/list", async () => [
      { name: "skill-a", description: "desc", source: "workspace" },
    ]);

    await expect(
      host.sessions.prompt({
        workspace_id: "ws-1",
        session_id: "sess-1",
        message_id: "message-1",
        idempotency_key: "idempotency-1",
        message: "hello",
        runtime: {
          provider: "codex",
          model: "gpt-5.6",
          reasoning_effort: "high",
          speed: "fast",
        },
      })
    ).resolves.toEqual({
      status: "accepted",
      turn_id: "turn-1",
      message_id: "message-1",
      idempotency_key: "idempotency-1",
      replayed: false,
    });
    await expect(
      host.sessions.stop({ workspace_id: "ws-1", session_id: "sess-1" })
    ).resolves.toEqual({});
    await expect(
      host.sessions.archive({ workspace_id: "ws-1", session_id: "sess-1" })
    ).resolves.toMatchObject({ session_id: "sess-1", archived_at: "2026-04-10T12:02:00.000Z" });
    await expect(
      host.sessions.unarchive({ workspace_id: "ws-1", session_id: "sess-1" })
    ).resolves.toMatchObject({ session_id: "sess-1", archived_at: null });
    await expect(
      host.sessions.status({ workspace_id: "ws-1", session_id: "sess-1" })
    ).resolves.toMatchObject({
      session_id: "sess-1",
      state: "active",
    });
    await expect(host.observe.health()).resolves.toMatchObject({ status: "ok" });
    await expect(host.skills.list()).resolves.toEqual([
      { name: "skill-a", description: "desc", source: "workspace" },
    ]);
  });

  it("resources helpers validate payload shape and send snapshot requests", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("resources/list", async params => {
      expect(params).toEqual({
        kind: "tool",
        scope: { kind: "workspace", id: "ws-1" },
        limit: 5,
      });
      return [
        {
          kind: "tool",
          id: "grep",
          version: 2,
          scope: { kind: "workspace", id: "ws-1" },
          owner: { kind: "extension", id: "resource-ext" },
          source: { kind: "extension", id: "resource-ext" },
          spec: { command: "rg" },
          created_at: "2026-04-15T12:00:00.000Z",
          updated_at: "2026-04-15T12:01:00.000Z",
        },
      ];
    });
    pair.host.handle("resources/get", async params => {
      expect(params).toEqual({ kind: "tool", id: "grep" });
      return {
        kind: "tool",
        id: "grep",
        version: 2,
        scope: { kind: "workspace", id: "ws-1" },
        owner: { kind: "extension", id: "resource-ext" },
        source: { kind: "extension", id: "resource-ext" },
        spec: { command: "rg" },
        created_at: "2026-04-15T12:00:00.000Z",
        updated_at: "2026-04-15T12:01:00.000Z",
      };
    });
    pair.host.handle("resources/snapshot", async params => {
      expect(params).toEqual({
        source_version: 3,
        records: [
          {
            kind: "tool",
            id: "grep",
            scope: { kind: "workspace", id: "ws-1" },
            spec: { command: "rg" },
          },
        ],
      });
      return {};
    });

    await expect(
      host.resources.list({
        kind: "tool",
        scope: { kind: "workspace", id: "ws-1" },
        limit: 5,
      })
    ).resolves.toHaveLength(1);
    await expect(host.resources.get({ kind: "tool", id: "grep" })).resolves.toMatchObject({
      id: "grep",
      version: 2,
    });
    await expect(
      host.resources.snapshot({
        source_version: 3,
        records: [
          {
            kind: "tool",
            id: "grep",
            scope: { kind: "workspace", id: "ws-1" },
            spec: { command: "rg" },
          },
        ],
      })
    ).resolves.toEqual({});

    await expect(
      host.resources.list({ scope: { kind: "workspace", id: "" } })
    ).rejects.toBeInstanceOf(InvalidParamsError);
    await expect(host.resources.get({ kind: "", id: "grep" })).rejects.toBeInstanceOf(
      InvalidParamsError
    );
    await expect(
      host.resources.snapshot({
        source_version: 0,
        records: [],
      })
    ).rejects.toBeInstanceOf(InvalidParamsError);
  });

  it("resources helpers surface 403, 409, 413, and 429 protocol errors", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("resources/get", async () => {
      throw new RPCError(403, "Forbidden", { error: "same-source only" });
    });
    pair.host.handle("resources/list", async () => {
      throw new RPCError(409, "Conflict", { error: "stale source_version" });
    });
    pair.host.handle("resources/snapshot", async params => {
      const request = params as { source_version: number };
      switch (request.source_version) {
        case 4:
          throw new RPCError(413, "Payload too large", { error: "snapshot too large" });
        case 5:
          throw new RPCError(429, "Rate limited", { error: "snapshot queued" });
        default:
          return {};
      }
    });

    await expect(host.resources.get({ kind: "tool", id: "grep" })).rejects.toMatchObject({
      code: 403,
      message: "Forbidden",
    });
    await expect(host.resources.list()).rejects.toMatchObject({
      code: 409,
      message: "Conflict",
    });
    await expect(
      host.resources.snapshot({
        source_version: 4,
        records: [
          {
            kind: "tool",
            id: "grep",
            scope: { kind: "workspace", id: "ws-1" },
            spec: { command: "rg" },
          },
        ],
      })
    ).rejects.toMatchObject({
      code: 413,
      message: "Payload too large",
    });
    await expect(
      host.resources.snapshot({
        source_version: 5,
        records: [
          {
            kind: "tool",
            id: "grep",
            scope: { kind: "workspace", id: "ws-1" },
            spec: { command: "rg" },
          },
        ],
      })
    ).rejects.toMatchObject({
      code: 429,
      message: "Rate limited",
    });
  });

  it("rejects calls before the session is ready", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => false });

    await expect(host.sessions.list()).rejects.toBeInstanceOf(NotInitializedError);
  });

  it("throws a typed capability denied error with code -32001", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("sessions/create", async () => {
      throw new CapabilityDeniedError({
        method: "sessions/create",
        required: ["session.write"],
        granted: ["session.read"],
      });
    });

    await expect(host.sessions.create({ agent: "coder" })).rejects.toMatchObject({
      code: -32001,
      data: {
        method: "sessions/create",
        required: ["session.write"],
        granted: ["session.read"],
      },
    });
  });

  it("throws a typed rate limited error with retry_after_ms", async () => {
    const pair = createMockTransportPair();
    const host = new HostAPI(pair.extension, { isReady: () => true });

    pair.host.handle("sessions/list", async () => {
      throw new RateLimitedError({
        scope: "host_api.sessions/list",
        retry_after_ms: 1000,
        limit: 10,
        burst: 20,
      });
    });

    await expect(host.sessions.list()).rejects.toMatchObject({
      code: -32002,
      data: {
        scope: "host_api.sessions/list",
        retry_after_ms: 1000,
      },
    });
  });
});
