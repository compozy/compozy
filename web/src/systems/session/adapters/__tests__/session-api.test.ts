import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  expectFetchRequest,
  fetchRequest,
  mockEmptyResponse,
  mockJsonResponse,
} from "@/test/fetch-test-utils";

import {
  SessionApiError,
  SessionLedgerUnavailableError,
  SessionNotFoundError,
  buildSessionStreamUrl,
  cancelSessionPrompt,
  clearSessionConversation,
  createSession,
  deleteSession,
  fetchSession,
  fetchSessionById,
  fetchSessionEvents,
  fetchSessionLedger,
  fetchSessionRecap,
  fetchSessionTranscript,
  fetchSessions,
  repairSession,
  resumeSession,
  sendSessionPrompt,
  stopSession,
} from "../session-api";

const mockSession = {
  id: "sess-001",
  name: "Test Session",
  agent_name: "claude-agent",
  workspace_id: "ws_alpha",
  workspace_path: "/tmp/test",
  state: "active",
  created_at: "2026-04-01T00:00:00Z",
  updated_at: "2026-04-01T01:00:00Z",
};
const WORKSPACE_ID = mockSession.workspace_id;

const mockRepair = {
  session_id: "sess-001",
  issues: [
    {
      code: "event_sequence_gap",
      severity: "warning",
      turn_id: "turn-1",
      detail: "gap before sequence 4",
    },
  ],
  actions: [
    {
      code: "append_terminal_error",
      turn_id: "turn-1",
      event_id: "ev-repair-1",
      persisted: false,
    },
  ],
  persisted: false,
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("fetchSessions", () => {
  it("preserves the counted page envelope", async () => {
    const sessions = [mockSession, { ...mockSession, id: "sess-002", name: "Second" }];
    const response = {
      sessions,
      page: { has_more: true, limit: 2, next_cursor: "cursor-2", total: 205 },
    };
    mockJsonResponse(response);

    const result = await fetchSessions();

    expect(result).toEqual(response);
    expect(result.page.total).toBe(205);
    await expectFetchRequest({ path: "/api/sessions" });
  });

  it("passes abort signal to fetch", async () => {
    mockJsonResponse({ sessions: [], page: { has_more: false, limit: 50, total: 0 } });

    const controller = new AbortController();
    await fetchSessions({}, controller.signal);

    await expectFetchRequest({
      path: "/api/sessions",
      signal: controller.signal,
    });
  });

  it("forwards stable filters and the opaque cursor", async () => {
    mockJsonResponse({ sessions: [], page: { has_more: false, limit: 25, total: 0 } });

    await fetchSessions({
      workspace: "ws_alpha",
      agent: "claude-agent",
      state: "active",
      resumable: true,
      sort: "last_activity",
      cursor: "cursor-1",
      limit: 25,
    });

    const url = new URL(fetchRequest().url);
    expect(url.pathname).toBe("/api/sessions");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      agent: "claude-agent",
      cursor: "cursor-1",
      limit: "25",
      resumable: "true",
      sort: "last_activity",
      state: "active",
      workspace: "ws_alpha",
    });
  });

  it("omits blank and undefined filters", async () => {
    mockJsonResponse({ sessions: [], page: { has_more: false, limit: 50, total: 0 } });

    await fetchSessions({ workspace: "   ", q: undefined });

    await expectFetchRequest({ path: "/api/sessions" });
  });

  it("throws on non-ok response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(fetchSessions()).rejects.toThrow("Failed to fetch sessions: 500");
  });

  it("returns empty array when server returns empty list", async () => {
    const response = { sessions: [], page: { has_more: false, limit: 50, total: 0 } };
    mockJsonResponse(response);

    const result = await fetchSessions();

    expect(result).toEqual(response);
  });
});

describe("buildSessionStreamUrl", () => {
  it("uses the cache fence for reconnects without the removed replay parameter", () => {
    const url = new URL(
      buildSessionStreamUrl(WORKSPACE_ID, "sess-001", {
        afterSequence: 18,
        epoch: 3,
        generation: 7,
      }),
      "http://localhost"
    );

    expect(url.searchParams.get("frames")).toBe("transcript");
    expect(url.searchParams.get("after_sequence")).toBe("18");
    expect(url.searchParams.get("epoch")).toBe("3");
    expect(url.searchParams.get("generation")).toBe("7");
    expect(url.searchParams.has("replay")).toBe(false);
  });
});

describe("createSession", () => {
  it("sends correct POST body with agent_name", async () => {
    mockJsonResponse({ session: mockSession });

    const result = await createSession({ agent_name: "claude-agent" });

    expect(result).toEqual(mockSession);
    await expectFetchRequest({
      body: { agent_name: "claude-agent" },
      method: "POST",
      path: "/api/sessions",
    });
  });

  it("sends the staged first message verbatim in the create body", async () => {
    mockJsonResponse({ session: mockSession });

    await createSession({
      agent_name: "claude-agent",
      prompt: "Draft the release notes.",
      workspace: "ws_alpha",
    });

    await expectFetchRequest({
      body: {
        agent_name: "claude-agent",
        prompt: "Draft the release notes.",
        workspace: "ws_alpha",
      },
      method: "POST",
      path: "/api/sessions",
    });
  });

  it("allows create session without agent_name", async () => {
    mockJsonResponse({ session: mockSession });

    await createSession({});

    await expectFetchRequest({
      body: {},
      method: "POST",
      path: "/api/sessions",
    });
  });

  it("sends optional name and workspace", async () => {
    mockJsonResponse({ session: mockSession });

    await createSession({
      agent_name: "claude-agent",
      name: "My Session",
      workspace: "/home",
    });

    await expectFetchRequest({
      body: {
        agent_name: "claude-agent",
        name: "My Session",
        workspace: "/home",
      },
      method: "POST",
      path: "/api/sessions",
    });
  });

  it("sends workspace_path when creating from an explicit path", async () => {
    mockJsonResponse({ session: mockSession });

    await createSession({
      agent_name: "claude-agent",
      workspace_path: "/workspace/demo",
    });

    await expectFetchRequest({
      body: {
        agent_name: "claude-agent",
        workspace_path: "/workspace/demo",
      },
      method: "POST",
      path: "/api/sessions",
    });
  });

  it("throws 'Max sessions reached' on 409", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 409 }));

    await expect(createSession({ agent_name: "claude-agent" })).rejects.toThrow(
      "Max sessions reached"
    );
  });

  it("throws generic error for other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(createSession({ agent_name: "claude-agent" })).rejects.toThrow(
      "Failed to create session: 500"
    );
  });
});

describe("fetchSession", () => {
  it("returns single SessionPayload on success", async () => {
    mockJsonResponse({ session: mockSession });

    const result = await fetchSession(WORKSPACE_ID, "sess-001");

    expect(result).toEqual(mockSession);
    await expectFetchRequest({ path: "/api/workspaces/ws_alpha/sessions/sess-001" });
  });

  it("throws 404 error for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(fetchSession(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });

  it("encodes session id in URL", async () => {
    mockJsonResponse({ session: mockSession });

    await fetchSession(WORKSPACE_ID, "id with spaces");

    await expectFetchRequest({ path: "/api/workspaces/ws_alpha/sessions/id%20with%20spaces" });
  });
});

describe("fetchSessionById", () => {
  it("returns a single SessionPayload from the direct session endpoint", async () => {
    mockJsonResponse({ session: mockSession });

    const result = await fetchSessionById("sess-001");

    expect(result).toEqual(mockSession);
    await expectFetchRequest({ path: "/api/sessions/sess-001" });
  });

  it("throws 404 error for an unknown direct session id", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(fetchSessionById("unknown")).rejects.toThrow("Session not found: unknown");
  });
});

describe("stopSession", () => {
  it("calls POST stop endpoint", async () => {
    mockEmptyResponse();

    await stopSession(WORKSPACE_ID, "sess-001");

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/stop",
    });
  });

  it("throws 404 error for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(stopSession(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });

  it("throws generic error for other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(stopSession(WORKSPACE_ID, "sess-001")).rejects.toThrow(
      'Failed to stop session "sess-001": 500'
    );
  });
});

describe("deleteSession", () => {
  it("calls DELETE endpoint", async () => {
    mockEmptyResponse();

    await deleteSession(WORKSPACE_ID, "sess-001");

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/workspaces/ws_alpha/sessions/sess-001",
    });
  });

  it("throws 404 error for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(deleteSession(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });

  it("throws generic error for other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(deleteSession(WORKSPACE_ID, "sess-001")).rejects.toThrow(
      'Failed to delete session "sess-001": 500'
    );
  });
});

describe("cancelSessionPrompt", () => {
  it("calls POST prompt cancel endpoint", async () => {
    mockEmptyResponse();

    await cancelSessionPrompt(WORKSPACE_ID, "sess-001");

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/prompt/cancel",
    });
  });

  it("throws 404 error for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(cancelSessionPrompt(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });

  it("passes abort signal to fetch", async () => {
    mockEmptyResponse();

    const controller = new AbortController();
    await cancelSessionPrompt(WORKSPACE_ID, "sess-001", controller.signal);

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/prompt/cancel",
      signal: controller.signal,
    });
  });
});

describe("sendSessionPrompt", () => {
  it("returns a direct Goal result without requiring a prompt envelope", async () => {
    const goalResult = {
      outcome: "cleared" as const,
      reason_code: null,
      replaced_run_id: null,
      snapshot: null,
    };
    mockJsonResponse(goalResult);

    const result = await sendSessionPrompt(WORKSPACE_ID, "sess-001", {
      message: "/goal clear",
    });

    expect(result).toEqual(goalResult);
    await expectFetchRequest({
      body: { message: "/goal clear" },
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/prompt",
    });
  });
});

describe("resumeSession", () => {
  it("calls POST attach endpoint", async () => {
    mockJsonResponse({ session: { ...mockSession, state: "active" } });

    const result = await resumeSession(WORKSPACE_ID, "sess-001");

    expect(result.state).toBe("active");
    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/attach",
    });
  });

  it("throws 404 error for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(resumeSession(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });
});

describe("fetchSessionRecap", () => {
  it("calls GET recap endpoint with limit", async () => {
    const recap = {
      session: mockSession,
      recent_markers: [],
      recent_messages: [],
      pending_inputs: 0,
      pending_markers: 0,
      snapshot: {
        generated_at: "2026-04-01T01:00:00Z",
        event_cursor: 4,
        transcript_cursor: 4,
        queue_generation: 0,
        consistency: "read_snapshot",
      },
    };
    mockJsonResponse({ recap });

    const result = await fetchSessionRecap(WORKSPACE_ID, "sess-001", 5);

    expect(result).toEqual(recap);
    const request = fetchRequest();
    const url = new URL(request.url);
    expect(request.method).toBe("GET");
    expect(url.pathname).toBe("/api/workspaces/ws_alpha/sessions/sess-001/recap");
    expect(url.searchParams.get("limit")).toBe("5");
  });
});

describe("repairSession", () => {
  it("calls POST repair endpoint with query flags and returns the repair payload", async () => {
    mockJsonResponse({ repair: mockRepair });

    const result = await repairSession(WORKSPACE_ID, "sess-001", {
      dry_run: true,
      force: true,
    });

    expect(result).toEqual(mockRepair);
    const request = fetchRequest();
    const url = new URL(request.url);
    expect(request.method).toBe("POST");
    expect(url.pathname).toBe("/api/workspaces/ws_alpha/sessions/sess-001/repair");
    expect(url.searchParams.get("dry_run")).toBe("true");
    expect(url.searchParams.get("force")).toBe("true");
  });

  it("throws 404 error for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(repairSession(WORKSPACE_ID, "unknown")).rejects.toBeInstanceOf(
      SessionNotFoundError
    );
    await expect(repairSession(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });

  it("throws typed adapter error for non-404 failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(repairSession(WORKSPACE_ID, "sess-001")).rejects.toBeInstanceOf(SessionApiError);
    await expect(repairSession(WORKSPACE_ID, "sess-001")).rejects.toMatchObject({
      message: 'Failed to repair session "sess-001": 500',
      status: 500,
      sessionId: "sess-001",
    });
  });

  it("passes abort signal to fetch", async () => {
    mockJsonResponse({ repair: mockRepair });

    const controller = new AbortController();
    await repairSession(WORKSPACE_ID, "sess-001", {}, controller.signal);

    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/repair",
      signal: controller.signal,
    });
  });
});

describe("clearSessionConversation", () => {
  it("calls POST clear endpoint and returns the refreshed session payload", async () => {
    mockJsonResponse({ session: mockSession });

    const result = await clearSessionConversation(WORKSPACE_ID, "sess-001");

    expect(result).toEqual(mockSession);
    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/sessions/sess-001/clear",
    });
  });

  it("throws 409 when a prompt is still running", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 409 }));

    await expect(clearSessionConversation(WORKSPACE_ID, "sess-001")).rejects.toThrow(
      'Cannot clear session "sess-001" while a prompt is still running'
    );
  });

  it("throws on invalid response payload", async () => {
    mockJsonResponse({ nope: true });

    await expect(clearSessionConversation(WORKSPACE_ID, "sess-001")).rejects.toThrow(
      'Failed to clear session "sess-001": invalid response payload'
    );
  });
});

describe("fetchSessionEvents", () => {
  const mockEvents = [
    {
      id: "evt-1",
      session_id: "sess-001",
      sequence: 1,
      turn_id: "turn-1",
      type: "agent_message",
      agent_name: "claude-agent",
      content: { text: "Hello" },
      timestamp: "2026-04-01T00:00:00Z",
    },
  ];

  it("returns parsed events array", async () => {
    mockJsonResponse({ events: mockEvents });

    const result = await fetchSessionEvents(WORKSPACE_ID, "sess-001");

    expect(result).toEqual(mockEvents);
    await expectFetchRequest({ path: "/api/workspaces/ws_alpha/sessions/sess-001/events" });
  });

  it("passes query params correctly", async () => {
    mockJsonResponse({ events: [] });

    await fetchSessionEvents(WORKSPACE_ID, "sess-001", {
      since: "2026-04-01T00:00:00Z",
      limit: 50,
      after_sequence: 10,
      type: "agent_message",
      agent_name: "claude-agent",
      turn_id: "turn-1",
    });

    const request = fetchRequest();
    const url = new URL(request.url);
    expect(url.pathname).toBe("/api/workspaces/ws_alpha/sessions/sess-001/events");
    expect(url.searchParams.get("since")).toBe("2026-04-01T00:00:00Z");
    expect(url.searchParams.get("limit")).toBe("50");
    expect(url.searchParams.get("after_sequence")).toBe("10");
    expect(url.searchParams.get("type")).toBe("agent_message");
    expect(url.searchParams.get("agent_name")).toBe("claude-agent");
    expect(url.searchParams.get("turn_id")).toBe("turn-1");
  });

  it("omits undefined params", async () => {
    mockJsonResponse({ events: [] });

    await fetchSessionEvents(WORKSPACE_ID, "sess-001", { limit: 10 });

    const request = fetchRequest();
    const url = new URL(request.url);
    expect(url.searchParams.get("limit")).toBe("10");
    expect(url.searchParams.has("since")).toBe(false);
    expect(url.searchParams.has("type")).toBe(false);
  });

  it("throws 404 for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(fetchSessionEvents(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });
});

describe("fetchSessionLedger", () => {
  const mockLedger = {
    meta: {
      version: 1,
      session_id: "sess-001",
      workspace_id: "ws_alpha",
      root_session_id: "sess-root",
      parent_session_id: "sess-parent",
      spawn_depth: 1,
      path: "/sessions/ws_alpha/sess-001/ledger.jsonl",
      checksum: "sha256:abc",
      created_at: "2026-04-20T10:00:00Z",
      stopped_at: "2026-04-20T11:00:00Z",
    },
    events: [
      { sequence: 1, event_type: "session.started", emitted_at: "2026-04-20T10:00:00Z" },
      { sequence: 2, event_type: "memory.recall", emitted_at: "2026-04-20T10:01:00Z" },
    ],
  };

  it("returns the materialized ledger response on success", async () => {
    mockJsonResponse(mockLedger);

    const result = await fetchSessionLedger("ws-alpha", "sess-001");

    expect(result).toEqual(mockLedger);
    await expectFetchRequest({ path: "/api/workspaces/ws-alpha/memory/sessions/sess-001/ledger" });
  });

  it("throws SessionLedgerUnavailableError when the ledger has not materialized (404)", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(fetchSessionLedger("ws-alpha", "sess-001")).rejects.toBeInstanceOf(
      SessionLedgerUnavailableError
    );
    await expect(fetchSessionLedger("ws-alpha", "sess-001")).rejects.toMatchObject({
      message: "Session ledger not materialized: sess-001",
      status: 404,
      sessionId: "sess-001",
    });
  });

  it("throws a typed adapter error for non-404 failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(fetchSessionLedger("ws-alpha", "sess-001")).rejects.toBeInstanceOf(
      SessionApiError
    );
    await expect(fetchSessionLedger("ws-alpha", "sess-001")).rejects.toMatchObject({
      message: 'Failed to fetch session ledger "sess-001": 500',
      status: 500,
      sessionId: "sess-001",
    });
  });

  it("passes the abort signal through to fetch", async () => {
    mockJsonResponse(mockLedger);

    const controller = new AbortController();
    await fetchSessionLedger("ws-alpha", "sess-001", controller.signal);

    await expectFetchRequest({
      path: "/api/workspaces/ws-alpha/memory/sessions/sess-001/ledger",
      signal: controller.signal,
    });
  });
});

describe("fetchSessionTranscript", () => {
  const mockTranscript = {
    epoch: 2,
    generation: 4,
    max_sequence: 12,
    has_older: true,
    next_before_sequence: 8,
    limit: 2,
    entries: [
      {
        sequence: 1,
        start_sequence: 1,
        message: {
          id: "evt-1",
          role: "assistant",
          parts: [{ type: "text", text: "Hello", state: "done" }],
        },
      },
      {
        sequence: 2,
        start_sequence: 2,
        message: {
          id: "tool-1",
          role: "assistant",
          parts: [
            {
              type: "tool-Read",
              toolCallId: "tool-1",
              state: "output-available",
              input: { file_path: "/tmp/file.ts" },
              output: {
                type: "tool_result",
                title: "Read",
                raw: { stdout: "done", file_path: "/tmp/file.ts" },
              },
            },
          ],
        },
      },
    ],
  };

  it("preserves transcript fences, page metadata, and stable entry sequences", async () => {
    mockJsonResponse(mockTranscript);

    const result = await fetchSessionTranscript(WORKSPACE_ID, "sess-001", {
      before_sequence: 20,
      limit: 2,
    });

    expect(result).toEqual(mockTranscript);
    expect(result.entries.map(entry => entry.start_sequence)).toEqual([1, 2]);
    const url = new URL(fetchRequest().url);
    expect(url.pathname).toBe("/api/workspaces/ws_alpha/sessions/sess-001/transcript");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      before_sequence: "20",
      limit: "2",
    });
  });

  it("returns an empty transcript without treating it as an invalid AI SDK message list", async () => {
    const empty = {
      entries: [],
      epoch: 1,
      generation: 1,
      max_sequence: 0,
      has_older: false,
      limit: 200,
    };
    mockJsonResponse(empty);

    const result = await fetchSessionTranscript(WORKSPACE_ID, "sess-001");

    expect(result).toEqual(empty);
    await expectFetchRequest({
      path: "/api/workspaces/ws_alpha/sessions/sess-001/transcript",
    });
  });

  it("returns AGH errored tool parts with raw output payloads", async () => {
    const blockedTranscript = {
      epoch: 1,
      generation: 1,
      max_sequence: 1,
      has_older: false,
      limit: 200,
      entries: [
        {
          sequence: 1,
          start_sequence: 1,
          message: {
            id: "blocked-turn",
            role: "assistant",
            parts: [
              {
                type: "tool-Attempt blocked terminal write",
                toolCallId: "tool-blocked-1",
                state: "output-error",
                input: { command: "touch browser-blocked.txt" },
                output: {
                  type: "tool_result",
                  title: "Attempt blocked terminal write",
                  error: "terminal/create denied before writing workspace marker",
                },
                errorText: "terminal/create denied before writing workspace marker",
              },
              {
                type: "text",
                text: "Sandbox blocked diagnostic: terminal/create denied before writing workspace marker.",
                state: "done",
              },
            ],
          },
        },
      ],
    };
    mockJsonResponse(blockedTranscript);

    const result = await fetchSessionTranscript(WORKSPACE_ID, "sess-001");

    expect(result).toEqual(blockedTranscript);
    expect(result.entries[0]?.message.parts).toHaveLength(2);
    expect(result.entries[0]?.message.parts?.[1]).toMatchObject({
      text: "Sandbox blocked diagnostic: terminal/create denied before writing workspace marker.",
      type: "text",
    });
  });

  it("throws 404 for unknown session", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(fetchSessionTranscript(WORKSPACE_ID, "unknown")).rejects.toThrow(
      "Session not found: unknown"
    );
  });
});
