// Suite: session-interactions-api
// Invariant: the Web reads restart-durable interaction records from the
// workspace-scoped interactions route with the daemon's explicit status filter,
// unwraps the envelope, propagates the abort signal, and maps failures to the
// typed session errors.
// Owning layer: adapter (systems/session/adapters)
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { fetchSessionInteractions } from "../session-interactions-api";

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

const expiredPermission = {
  interaction_id: "int-1",
  kind: "permission",
  provider_request_id: "req-1",
  turn_id: "turn-001",
  title: "Bash",
  status: "canceled",
  created_at: "2026-09-05T10:00:00Z",
  resolved_at: "2026-09-05T10:02:00Z",
  resolution: "failed-by-restart",
  resolved_by: "system",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("fetchSessionInteractions", () => {
  it("requests the scoped interactions path with the status filter and unwraps the envelope", async () => {
    mockJsonResponse({ interactions: [expiredPermission] });

    const result = await fetchSessionInteractions(WORKSPACE_ID, SESSION_ID, {
      status: "canceled",
    });

    expect(result).toEqual([expiredPermission]);
    await expectFetchRequest({
      method: "GET",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/interactions?status=canceled",
    });
  });

  it("leaves the status filter to the daemon default when none is given and passes the signal", async () => {
    mockJsonResponse({ interactions: [] });
    const controller = new AbortController();

    await fetchSessionInteractions(WORKSPACE_ID, SESSION_ID, { signal: controller.signal });

    await expectFetchRequest({
      method: "GET",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/interactions",
      signal: controller.signal,
    });
  });

  it("throws a not-found error for an unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(
      fetchSessionInteractions(WORKSPACE_ID, "unknown", { status: "canceled" })
    ).rejects.toThrow("Session not found: unknown");
  });

  it("throws a generic error when the daemon rejects the request", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 400 }));

    await expect(
      fetchSessionInteractions(WORKSPACE_ID, SESSION_ID, { status: "canceled" })
    ).rejects.toThrow(`Failed to fetch interactions for session "${SESSION_ID}": 400`);
  });
});
