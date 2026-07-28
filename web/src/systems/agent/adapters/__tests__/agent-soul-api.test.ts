import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { AgentDigestConflictError, isAgentDigestConflict } from "../agent-api";
import {
  deleteAgentSoul,
  fetchAgentSoul,
  fetchAgentSoulHistory,
  putAgentSoul,
  rollbackAgentSoul,
  validateAgentSoul,
} from "../agent-soul-api";

const soulPayload = {
  active: true,
  present: true,
  enabled: true,
  body: "Be helpful.",
  digest: "a".repeat(64),
  valid: true,
  validation_status: "valid" as const,
  frontmatter: {},
  limits: { max_body_bytes: 65536 },
  config_provenance: {
    digest: "a".repeat(64),
    enabled: true,
    context_projection_bytes: 0,
    max_body_bytes: 65536,
  },
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("agent-soul-api", () => {
  it("Should fetch soul with workspace_id query", async () => {
    mockJsonResponse(soulPayload);

    const result = await fetchAgentSoul("coder", "ws_alpha");

    expect(result).toEqual(soulPayload);
    await expectFetchRequest({ path: "/api/agents/coder/soul?workspace_id=ws_alpha" });
  });

  it("Should put soul and surface digest conflicts as typed errors", async () => {
    mockJsonResponse({
      soul: soulPayload,
      revision: {
        id: "r1",
        action: "put",
        actor: { kind: "user" },
        agent_name: "coder",
        created_at: "2026-01-01T00:00:00Z",
        source_path: "coder/SOUL.md",
      },
    });

    await putAgentSoul("coder", { body: "Be helpful.", expected_digest: "a".repeat(64) });

    await expectFetchRequest({
      path: "/api/agents/coder/soul",
      method: "PUT",
      body: { body: "Be helpful.", expected_digest: "a".repeat(64) },
    });

    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "soul digest conflict" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );

    await expect(putAgentSoul("coder", { body: "x", expected_digest: "stale" })).rejects.toSatisfy(
      (error: unknown) => error instanceof AgentDigestConflictError && isAgentDigestConflict(error)
    );
  });

  it("Should delete soul with a JSON body", async () => {
    mockJsonResponse({
      soul: { ...soulPayload, active: false, present: false, validation_status: "missing" },
      revision: {
        id: "r2",
        action: "delete",
        actor: { kind: "user" },
        agent_name: "coder",
        created_at: "2026-01-01T00:00:00Z",
        source_path: "coder/SOUL.md",
      },
    });

    await deleteAgentSoul("coder", { expected_digest: "a".repeat(64) });

    await expectFetchRequest({
      path: "/api/agents/coder/soul",
      method: "DELETE",
      body: { expected_digest: "a".repeat(64) },
    });
  });

  it("Should validate, list history, and rollback", async () => {
    mockJsonResponse(soulPayload);
    await validateAgentSoul("coder", { body: "Be helpful." });
    await expectFetchRequest({
      path: "/api/agents/coder/soul/validate",
      method: "POST",
      body: { body: "Be helpful." },
    });

    mockJsonResponse({ revisions: [] });
    await fetchAgentSoulHistory("coder", "ws_alpha");
    await expectFetchRequest({
      path: "/api/agents/coder/soul/history?workspace_id=ws_alpha",
      callIndex: 1,
    });

    mockJsonResponse({
      soul: soulPayload,
      revision: {
        id: "r3",
        action: "rollback",
        actor: { kind: "user" },
        agent_name: "coder",
        created_at: "2026-01-01T00:00:00Z",
        source_path: "coder/SOUL.md",
      },
    });
    await rollbackAgentSoul("coder", {
      revision_id: "r1",
      expected_digest: "a".repeat(64),
    });
    await expectFetchRequest({
      path: "/api/agents/coder/soul/rollback",
      method: "POST",
      callIndex: 2,
      body: { revision_id: "r1", expected_digest: "a".repeat(64) },
    });
  });

  it("Should throw generic errors for non-conflict failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(fetchAgentSoul("coder")).rejects.toThrow(/Failed to fetch soul/);

    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(
      putAgentSoul("coder", { body: "x", expected_digest: "a".repeat(64) })
    ).rejects.toThrow(/Failed to update soul/);

    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "soul digest conflict" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );
    await expect(deleteAgentSoul("coder", { expected_digest: "stale" })).rejects.toBeInstanceOf(
      AgentDigestConflictError
    );

    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    await expect(fetchAgentSoulHistory("coder")).rejects.toThrow(/Failed to fetch soul history/);

    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "soul digest conflict" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );
    await expect(
      rollbackAgentSoul("coder", { revision_id: "r1", expected_digest: "stale" })
    ).rejects.toBeInstanceOf(AgentDigestConflictError);
  });
});
