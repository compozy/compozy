import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { AgentDigestConflictError, isAgentDigestConflict } from "../agent-api";
import {
  deleteAgentHeartbeat,
  fetchAgentHeartbeat,
  fetchAgentHeartbeatHistory,
  fetchAgentHeartbeatStatus,
  putAgentHeartbeat,
  rollbackAgentHeartbeat,
  validateAgentHeartbeat,
  wakeAgentHeartbeat,
} from "../agent-heartbeat-api";

const heartbeatPayload = {
  active: true,
  present: true,
  enabled: true,
  valid: true,
  validation_status: "valid" as const,
  digest: "b".repeat(64),
  schema_version: 1,
  frontmatter: { enabled: true, version: 1, context: {}, preferences: {} },
  limits: { max_body_bytes: 65536 },
  preferences: { min_interval: "30m", context: {} },
  prompt: {
    active: true,
    context: {},
    max_body_bytes: 65536,
    max_bytes: 65536,
    preferences: { context: {}, min_interval: "30m" },
    truncated: false,
  },
  config_provenance: {
    digest: "b".repeat(64),
    subset: {
      active_session_only: false,
      allow_active_hours_preferences: true,
      context_projection_bytes: 0,
      default_interval: "30m",
      enabled: true,
      max_body_bytes: 65536,
      max_wakes_per_cycle: 1,
      min_interval: "5m",
      session_health_hook_min_interval: "1m",
      session_health_stale_after: "10m",
      wake_cooldown: "1m",
      wake_event_retention: "24h",
    },
  },
};

const revision = {
  id: "hb-r1",
  operation: "write" as const,
  actor: { kind: "user" as const },
  agent_name: "coder",
  created_at: "2026-01-01T00:00:00Z",
  source_path: "coder/HEARTBEAT.md",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("agent-heartbeat-api", () => {
  it("Should fetch heartbeat with workspace_id", async () => {
    mockJsonResponse(heartbeatPayload);

    const result = await fetchAgentHeartbeat("coder", "ws_alpha");

    expect(result).toEqual(heartbeatPayload);
    await expectFetchRequest({ path: "/api/agents/coder/heartbeat?workspace_id=ws_alpha" });
  });

  it("Should put heartbeat and type 409 as digest conflict", async () => {
    mockJsonResponse({ heartbeat: heartbeatPayload, revision });

    await putAgentHeartbeat("coder", {
      body: "---\nenabled: true\n---\n",
      expected_digest: "b".repeat(64),
    });

    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat",
      method: "PUT",
      body: { body: "---\nenabled: true\n---\n", expected_digest: "b".repeat(64) },
    });

    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "heartbeat digest conflict" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );

    await expect(
      putAgentHeartbeat("coder", { body: "x", expected_digest: "stale" })
    ).rejects.toSatisfy(
      (error: unknown) => error instanceof AgentDigestConflictError && isAgentDigestConflict(error)
    );
  });

  it("Should delete, validate, history, rollback, status, and wake", async () => {
    mockJsonResponse({
      heartbeat: { ...heartbeatPayload, active: false, enabled: false },
      revision: { ...revision, operation: "delete" as const },
    });
    await deleteAgentHeartbeat("coder", { expected_digest: "b".repeat(64) });
    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat",
      method: "DELETE",
      body: { expected_digest: "b".repeat(64) },
    });

    mockJsonResponse(heartbeatPayload);
    await validateAgentHeartbeat("coder", { body: "---\nenabled: true\n---\n" });
    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat/validate",
      method: "POST",
      callIndex: 1,
      body: { body: "---\nenabled: true\n---\n" },
    });

    mockJsonResponse({ revisions: [] });
    await fetchAgentHeartbeatHistory("coder", "ws_alpha");
    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat/history?workspace_id=ws_alpha",
      callIndex: 2,
    });

    mockJsonResponse({
      heartbeat: heartbeatPayload,
      revision: { ...revision, operation: "rollback" as const },
    });
    await rollbackAgentHeartbeat("coder", {
      revision_id: "hb-r1",
      expected_digest: "b".repeat(64),
    });
    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat/rollback",
      method: "POST",
      callIndex: 3,
      body: { revision_id: "hb-r1", expected_digest: "b".repeat(64) },
    });

    mockJsonResponse({
      agent_name: "coder",
      active: true,
      present: true,
      enabled: true,
      valid: true,
      validation_status: "valid",
      preferences: { min_interval: "30m", context: {} },
    });
    await fetchAgentHeartbeatStatus("coder", { workspaceId: "ws_alpha" });
    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat/status?workspace_id=ws_alpha",
      callIndex: 4,
    });

    mockJsonResponse({
      decision: { result: "sent", reason: "wake_sent", wake_event_id: "wake-1" },
    });
    await wakeAgentHeartbeat("coder", { session_id: "sess-1", source: "manual" });
    await expectFetchRequest({
      path: "/api/agents/coder/heartbeat/wake",
      method: "POST",
      callIndex: 5,
      body: { session_id: "sess-1", source: "manual" },
    });
  });

  it("Should throw generic errors for non-conflict failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(fetchAgentHeartbeat("coder")).rejects.toThrow(/Failed to fetch heartbeat/);

    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(
      putAgentHeartbeat("coder", { body: "x", expected_digest: "b".repeat(64) })
    ).rejects.toThrow(/Failed to update heartbeat/);

    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "heartbeat digest conflict" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );
    await expect(
      deleteAgentHeartbeat("coder", { expected_digest: "stale" })
    ).rejects.toBeInstanceOf(AgentDigestConflictError);

    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(fetchAgentHeartbeatHistory("coder")).rejects.toThrow(
      /Failed to fetch heartbeat history/
    );

    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "heartbeat digest conflict" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );
    await expect(
      rollbackAgentHeartbeat("coder", { expected_digest: "stale" })
    ).rejects.toBeInstanceOf(AgentDigestConflictError);

    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(fetchAgentHeartbeatStatus("coder")).rejects.toThrow(
      /Failed to fetch heartbeat status/
    );

    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(
      wakeAgentHeartbeat("coder", { session_id: "sess-1", source: "manual" })
    ).rejects.toThrow(/Failed to wake heartbeat/);
  });
});
