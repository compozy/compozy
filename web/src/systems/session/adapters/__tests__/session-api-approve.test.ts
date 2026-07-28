import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse } from "@/test/fetch-test-utils";

import { approveSession } from "../session-api";

const WORKSPACE_ID = "ws_alpha";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("approveSession", () => {
  it("sends correct POST body with request_id, turn_id, and decision", async () => {
    mockEmptyResponse();

    await approveSession(WORKSPACE_ID, "sess-001", {
      request_id: "req-123",
      turn_id: "turn-1",
      decision: "allow-once",
    });

    await expectFetchRequest({
      body: {
        request_id: "req-123",
        turn_id: "turn-1",
        decision: "allow-once",
      },
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/approve",
    });
  });

  it("sends allow-always decision", async () => {
    mockEmptyResponse();

    await approveSession(WORKSPACE_ID, "sess-001", {
      request_id: "req-123",
      turn_id: "",
      decision: "allow-always",
    });

    const request = await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/approve",
    });

    expect((await request.clone().json()).decision).toBe("allow-always");
  });

  it("sends reject-once decision", async () => {
    mockEmptyResponse();

    await approveSession(WORKSPACE_ID, "sess-001", {
      request_id: "req-123",
      turn_id: "",
      decision: "reject-once",
    });

    const request = await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/approve",
    });

    expect((await request.clone().json()).decision).toBe("reject-once");
  });

  it("sends reject-always decision", async () => {
    mockEmptyResponse();

    await approveSession(WORKSPACE_ID, "sess-001", {
      request_id: "req-123",
      turn_id: "",
      decision: "reject-always",
    });

    const request = await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/approve",
    });

    expect((await request.clone().json()).decision).toBe("reject-always");
  });

  it("throws 404 for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(
      approveSession(WORKSPACE_ID, "unknown", {
        request_id: "req-1",
        turn_id: "",
        decision: "allow-once",
      })
    ).rejects.toThrow("Session not found: unknown");
  });

  it("throws generic error for other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(
      approveSession(WORKSPACE_ID, "sess-001", {
        request_id: "req-1",
        turn_id: "",
        decision: "allow-once",
      })
    ).rejects.toThrow("Failed to approve permission: 500");
  });

  it("passes abort signal to fetch", async () => {
    mockEmptyResponse();

    const controller = new AbortController();
    await approveSession(
      WORKSPACE_ID,
      "sess-001",
      {
        request_id: "req-1",
        turn_id: "",
        decision: "allow-once",
      },
      controller.signal
    );

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/approve",
      signal: controller.signal,
    });
  });

  it("encodes session id in URL", async () => {
    mockEmptyResponse();

    await approveSession(WORKSPACE_ID, "id with spaces", {
      request_id: "req-1",
      turn_id: "",
      decision: "allow-once",
    });

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/id%20with%20spaces/approve",
    });
  });
});
