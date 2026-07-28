import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import {
  answerSessionClarification,
  ClarificationNotAnswerableError,
  fetchSessionClarifications,
} from "../session-clarification-api";

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("fetchSessionClarifications", () => {
  it("requests the scoped clarifications path and unwraps the envelope", async () => {
    const clarification = {
      agent_name: "mock",
      asked_at: "2026-07-16T00:00:00Z",
      choices: ["Fast", "Safe"],
      deadline: "2026-07-16T00:05:00Z",
      question: "Which path?",
      request_id: "req-1",
      session_id: SESSION_ID,
    };
    mockJsonResponse({ clarifications: [clarification] });

    const result = await fetchSessionClarifications(WORKSPACE_ID, SESSION_ID);

    expect(result).toEqual([clarification]);
    await expectFetchRequest({
      method: "GET",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clarifications",
    });
  });

  it("passes the abort signal to fetch", async () => {
    mockJsonResponse({ clarifications: [] });
    const controller = new AbortController();

    await fetchSessionClarifications(WORKSPACE_ID, SESSION_ID, controller.signal);

    await expectFetchRequest({
      method: "GET",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clarifications",
      signal: controller.signal,
    });
  });

  it("throws a not-found error for an unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(fetchSessionClarifications(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });

  it("throws a generic error when the broker is unavailable", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(fetchSessionClarifications(WORKSPACE_ID, SESSION_ID)).rejects.toThrow(
      `Failed to fetch clarifications for session "${SESSION_ID}": 503`
    );
  });
});

describe("answerSessionClarification", () => {
  it("posts a zero-based choice_index to the request path and returns the structured answer", async () => {
    mockJsonResponse({ choice: 1, fallback: false, text: "" });

    const result = await answerSessionClarification(WORKSPACE_ID, SESSION_ID, "req-1", {
      choice_index: 1,
    });

    expect(result).toEqual({ choice: 1, fallback: false, text: "" });
    await expectFetchRequest({
      body: { choice_index: 1 },
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clarifications/req-1/answer",
    });
  });

  it("posts a trimmed free-text answer", async () => {
    mockJsonResponse({ choice: null, fallback: false, text: "ship it" });

    const result = await answerSessionClarification(WORKSPACE_ID, SESSION_ID, "req-2", {
      text: "ship it",
    });

    expect(result).toEqual({ choice: null, fallback: false, text: "ship it" });
    await expectFetchRequest({
      body: { text: "ship it" },
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clarifications/req-2/answer",
    });
  });

  it("encodes the request id in the URL", async () => {
    mockJsonResponse({ choice: 0, fallback: false, text: "" });

    await answerSessionClarification(WORKSPACE_ID, SESSION_ID, "req id/1", { choice_index: 0 });

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clarifications/req%20id%2F1/answer",
    });
  });

  it("passes the abort signal to fetch", async () => {
    mockJsonResponse({ choice: 0, fallback: false, text: "" });
    const controller = new AbortController();

    await answerSessionClarification(
      WORKSPACE_ID,
      SESSION_ID,
      "req-1",
      { choice_index: 0 },
      controller.signal
    );

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clarifications/req-1/answer",
      signal: controller.signal,
    });
  });

  it("throws a not-answerable error on a 409 conflict", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 409 }));

    await expect(
      answerSessionClarification(WORKSPACE_ID, SESSION_ID, "req-gone", { text: "late" })
    ).rejects.toBeInstanceOf(ClarificationNotAnswerableError);
  });

  it("throws a not-found error on a 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(
      answerSessionClarification(WORKSPACE_ID, SESSION_ID, "req-1", { choice_index: 0 })
    ).rejects.toThrow(`Session not found: ${SESSION_ID}`);
  });

  it("throws a generic error for other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(
      answerSessionClarification(WORKSPACE_ID, SESSION_ID, "req-1", { choice_index: 0 })
    ).rejects.toThrow(`Failed to answer clarification "req-1" for session "${SESSION_ID}": 500`);
  });
});
