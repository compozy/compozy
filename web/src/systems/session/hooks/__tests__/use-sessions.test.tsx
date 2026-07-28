import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "../../types";
import { sessionKeys } from "../../lib/query-keys";
import { useSession, useSessionById, useSessions } from "../use-sessions";

vi.mock("../../adapters/session-api", () => ({
  fetchSession: vi.fn(),
  fetchSessionLedger: vi.fn(),
  fetchSessionRecap: vi.fn(),
  fetchSessions: vi.fn(),
  fetchSessionEvents: vi.fn(),
  fetchSessionGoal: vi.fn(),
  fetchSessionHistory: vi.fn(),
  fetchSessionTranscript: vi.fn(),
  SessionApiError: class SessionApiError extends Error {
    constructor(
      message: string,
      public readonly status: number,
      public readonly sessionId?: string
    ) {
      super(message);
      this.name = "SessionApiError";
    }
  },
  SessionLedgerUnavailableError: class SessionLedgerUnavailableError extends Error {},
  SessionNotFoundError: class SessionNotFoundError extends Error {
    constructor(public readonly sessionId: string) {
      super(`Session not found: ${sessionId}`);
      this.name = "SessionNotFoundError";
    }
  },
}));

import { fetchSession, fetchSessions } from "../../adapters/session-api";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeSession(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    id: "sess-001",
    agent_name: "claude-agent",
    provider: "claude",
    workspace_id: "ws_alpha",
    workspace_path: "/workspace/alpha",
    state: "active",
    badge: "idle",
    attachable: true,
    available_commands: [],
    created_at: "2026-04-06T10:00:00Z",
    updated_at: "2026-04-06T10:00:00Z",
    ...overrides,
  };
}

describe("useSessions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("loads sessions for the selected workspace filter", async () => {
    vi.mocked(fetchSessions).mockResolvedValue({
      sessions: [makeSession()],
      page: { has_more: false, limit: 50, total: 1 },
    });

    const { result } = renderHook(() => useSessions("ws_alpha"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(result.current.data?.[0]?.provider).toBe("claude");
    expect(result.current.total).toBe(1);
    expect(fetchSessions).toHaveBeenCalledWith({ workspace: "ws_alpha" }, expect.any(AbortSignal));
  });

  it("appends cursor pages while keeping stable filters in the base query key", async () => {
    vi.mocked(fetchSessions)
      .mockResolvedValueOnce({
        sessions: [makeSession({ id: "sess-002" })],
        page: { has_more: true, limit: 1, next_cursor: "cursor-1", total: 2 },
      })
      .mockResolvedValueOnce({
        sessions: [makeSession({ id: "sess-001" })],
        page: { has_more: false, limit: 1, total: 2 },
      });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(
      () =>
        useSessions("ws_alpha", {
          filters: { agent: "claude-agent", sort: "last_activity", limit: 1 },
        }),
      { wrapper: createWrapperWithClient(queryClient) }
    );

    await waitFor(() =>
      expect(result.current.data?.map(session => session.id)).toEqual(["sess-002"])
    );
    await act(async () => {
      await result.current.fetchNextPage();
    });

    await waitFor(() =>
      expect(result.current.data?.map(session => session.id)).toEqual(["sess-002", "sess-001"])
    );
    expect(result.current.total).toBe(2);
    expect(result.current.hasNextPage).toBe(false);
    expect(fetchSessions).toHaveBeenNthCalledWith(
      1,
      {
        agent: "claude-agent",
        limit: 1,
        sort: "last_activity",
        workspace: "ws_alpha",
      },
      expect.any(AbortSignal)
    );
    expect(fetchSessions).toHaveBeenNthCalledWith(
      2,
      {
        agent: "claude-agent",
        cursor: "cursor-1",
        limit: 1,
        sort: "last_activity",
        workspace: "ws_alpha",
      },
      expect.any(AbortSignal)
    );
    expect(
      queryClient
        .getQueryCache()
        .getAll()
        .map(query => query.queryKey)
    ).toContainEqual(
      sessionKeys.list({
        agent: "claude-agent",
        limit: 1,
        sort: "last_activity",
        workspace: "ws_alpha",
      })
    );
  });

  it("does not periodically refetch every loaded catalog page", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchSessions)
      .mockResolvedValueOnce({
        sessions: [makeSession({ id: "sess-002" })],
        page: { has_more: true, limit: 1, next_cursor: "cursor-1", total: 2 },
      })
      .mockResolvedValueOnce({
        sessions: [makeSession({ id: "sess-001" })],
        page: { has_more: false, limit: 1, total: 2 },
      });

    const { result } = renderHook(() => useSessions("ws_alpha", { filters: { limit: 1 } }), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(result.current.data?.map(session => session.id)).toEqual(["sess-002"]);
    await act(async () => {
      await result.current.fetchNextPage();
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(result.current.data?.map(session => session.id)).toEqual(["sess-002", "sess-001"]);

    expect(fetchSessions).toHaveBeenCalledTimes(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });
    expect(fetchSessions).toHaveBeenCalledTimes(2);
  });

  it("disables the query when the workspace filter is unavailable", async () => {
    renderHook(() => useSessions(null, { enabled: false }), {
      wrapper: createWrapper(),
    });

    expect(fetchSessions).not.toHaveBeenCalled();
  });
});

describe("session event query identity", () => {
  it("includes every bounded event response parameter after normalization", () => {
    const first = sessionKeys.eventsList("ws_alpha", "sess-001", {
      after_sequence: 10,
      agent_name: " claude-agent ",
      limit: 25,
      since: " 2026-07-11T12:00:00Z ",
      turn_id: " turn-1 ",
      type: " tool_call ",
    });
    const second = sessionKeys.eventsList("ws_alpha", "sess-001", {
      after_sequence: 11,
      agent_name: "claude-agent",
      limit: 25,
      since: "2026-07-11T12:00:00Z",
      turn_id: "turn-1",
      type: "tool_call",
    });

    expect(first).not.toEqual(second);
    expect(first.at(-1)).toEqual({
      after_sequence: 10,
      agent_name: "claude-agent",
      limit: 25,
      since: "2026-07-11T12:00:00Z",
      turn_id: "turn-1",
      type: "tool_call",
    });
    expect(sessionKeys.eventsList("ws_alpha", "sess-001", { type: "   " })).toEqual(
      sessionKeys.eventsList("ws_alpha", "sess-001")
    );
  });
});

describe("session clarification query identity", () => {
  it("selects distinct pending caches per workspace and per session", () => {
    const key = sessionKeys.clarifications("ws_alpha", "sess-001");
    expect(key).not.toEqual(sessionKeys.clarifications("ws_beta", "sess-001"));
    expect(key).not.toEqual(sessionKeys.clarifications("ws_alpha", "sess-002"));
    expect(sessionKeys.clarifications("ws_alpha", "sess-001")).toEqual(key);
  });
});

function createWrapperWithClient(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useSession", () => {
  it("loads a single session detail", async () => {
    vi.mocked(fetchSession).mockResolvedValue({
      id: "sess-001",
      agent_name: "claude-agent",
      provider: "claude",
      workspace_id: "ws_alpha",
      workspace_path: "/workspace/alpha",
      state: "active",
      badge: "idle",
      attachable: true,
      available_commands: [],
      created_at: "2026-04-06T10:00:00Z",
      updated_at: "2026-04-06T10:00:00Z",
    });

    const { result } = renderHook(() => useSession("sess-001", "ws_alpha"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.id).toBe("sess-001");
    });

    expect(result.current.data?.provider).toBe("claude");
    expect(fetchSession).toHaveBeenCalledWith("ws_alpha", "sess-001", expect.any(AbortSignal));
  });
});

describe("useSessionById", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("resolves session detail without issuing a full session-list request", async () => {
    vi.mocked(fetchSession).mockResolvedValue(
      makeSession({ id: "sess-001", workspace_id: "ws_alpha", name: "Detailed session" })
    );

    const { result } = renderHook(() => useSessionById("sess-001", "ws_alpha"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.name).toBe("Detailed session");
    });

    expect(fetchSessions).not.toHaveBeenCalled();
    expect(fetchSession).toHaveBeenCalledWith("ws_alpha", "sess-001", expect.any(AbortSignal));
  });

  it("reports detail not found without falling back to the full session list", async () => {
    vi.mocked(fetchSession).mockRejectedValue(new Error("Session not found: missing-session"));

    const { result } = renderHook(() => useSessionById("missing-session", "ws_alpha"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.error?.message).toBe("Session not found: missing-session");
    });

    expect(fetchSessions).not.toHaveBeenCalled();
    expect(fetchSession).toHaveBeenCalledWith(
      "ws_alpha",
      "missing-session",
      expect.any(AbortSignal)
    );
  });
});
