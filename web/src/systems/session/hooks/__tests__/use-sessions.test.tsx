import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionPayload } from "../../types";
import { sessionKeys } from "../../lib/query-keys";
import { useSession, useSessionById, useSessionLedger, useSessions } from "../use-sessions";
import {
  fetchSessionLedger,
  SessionLedgerUnavailableError,
  fetchSessions,
} from "../../adapters/session-api";
import { fetchSessionById } from "../../adapters/session-owner-api";
import { useSessionContext, useSessionUsageTurns } from "../use-session-context";
import { fetchSessionUsage, fetchSessionUsageTurns } from "../../adapters/session-api";
import { sessionUsageOptions, sessionUsageTurnsOptions } from "../../lib/query-options";
import {
  sessionContextUsageFixture,
  sessionContextTurnsFixture,
} from "../../mocks/context-fixtures";
import type { SessionUsagePayload } from "../../types";

vi.mock("../../adapters/session-api", async importOriginal => ({
  fetchSessionLedger: vi.fn(),
  fetchSessionRecap: vi.fn(),
  fetchSessionUsage: vi.fn(),
  fetchSessionUsageTurns: vi.fn(),
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
  SessionLedgerUnavailableError: (
    await importOriginal<typeof import("../../adapters/session-api")>()
  ).SessionLedgerUnavailableError,
  SessionNotFoundError: class SessionNotFoundError extends Error {
    constructor(public readonly sessionId: string) {
      super(`Session not found: ${sessionId}`);
      this.name = "SessionNotFoundError";
    }
  },
}));

vi.mock("../../adapters/session-owner-api", () => ({
  fetchSessionById: vi.fn(),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeSession(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    supervision: null,
    profile_name: "default",
    profile_id: "00000000000000000000000000",
    id: "sess-001",
    agent_name: "claude-agent",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "claude" },
      selection_revision: 0,
    },
    workspace_id: "ws_alpha",
    workspace_path: "/workspace/alpha",
    state: "active",
    badge: "idle",
    attachable: true,
    available_commands: [],
    created_at: "2026-04-06T10:00:00Z",
    updated_at: "2026-04-06T10:00:00Z",
    ...overrides,
    archived_at: overrides.archived_at ?? null,
    pending_interactions: overrides.pending_interactions ?? [],
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

    expect(result.current.data?.[0]?.runtime.effective?.provider).toBe("claude");
    expect(result.current.total).toBe(1);
    // Every session read carries its profile scope: the daemon has exactly two
    // read modes and an omitted scope silently resolves to `default`.
    expect(fetchSessions).toHaveBeenCalledWith(
      { profile: "default", workspace_id: "ws_alpha" },
      expect.any(AbortSignal)
    );
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
        profile: "default",
        sort: "last_activity",
        workspace_id: "ws_alpha",
      },
      expect.any(AbortSignal)
    );
    expect(fetchSessions).toHaveBeenNthCalledWith(
      2,
      {
        agent: "claude-agent",
        cursor: "cursor-1",
        limit: 1,
        profile: "default",
        sort: "last_activity",
        workspace_id: "ws_alpha",
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
        profile: "default",
        sort: "last_activity",
        workspace_id: "ws_alpha",
      })
    );
  });

  it("loads every cursor page when the consumer requests the complete catalog", async () => {
    vi.mocked(fetchSessions)
      .mockResolvedValueOnce({
        sessions: [makeSession({ id: "sess-002" })],
        page: { has_more: true, limit: 1, next_cursor: "cursor-1", total: 2 },
      })
      .mockResolvedValueOnce({
        sessions: [makeSession({ id: "sess-001" })],
        page: { has_more: false, limit: 1, total: 2 },
      });

    const { result } = renderHook(
      () =>
        useSessions("ws_alpha", {
          loadAll: true,
          filters: { limit: 1, sort: "attention" },
        }),
      { wrapper: createWrapper() }
    );

    await waitFor(() =>
      expect(result.current.data?.map(session => session.id)).toEqual(["sess-002", "sess-001"])
    );
    expect(fetchSessions).toHaveBeenCalledTimes(2);
    expect(result.current.hasNextPage).toBe(false);
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
    vi.mocked(fetchSessionById).mockResolvedValue({
      supervision: null,
      profile_id: "00000000000000000000000000",
      profile_name: "default",
      id: "sess-001",
      agent_name: "claude-agent",
      runtime: {
        status: "ready",
        transition: "initial_bind",
        effective: { provider: "claude" },
        selection_revision: 0,
      },
      workspace_id: "ws_alpha",
      workspace_path: "/workspace/alpha",
      state: "active",
      badge: "idle",
      attachable: true,
      archived_at: null,
      available_commands: [],
      pending_interactions: [],
      created_at: "2026-04-06T10:00:00Z",
      updated_at: "2026-04-06T10:00:00Z",
    });

    const { result } = renderHook(() => useSession("sess-001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.id).toBe("sess-001");
    });

    expect(result.current.data?.runtime.effective?.provider).toBe("claude");
    expect(fetchSessionById).toHaveBeenCalledWith(
      "sess-001",
      { profile: "default" },
      expect.any(AbortSignal)
    );
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
    vi.mocked(fetchSessionById).mockResolvedValue(
      makeSession({ id: "sess-001", workspace_id: "ws_alpha", name: "Detailed session" })
    );

    const { result } = renderHook(() => useSessionById("sess-001", "ws_alpha"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.name).toBe("Detailed session");
    });

    expect(fetchSessions).not.toHaveBeenCalled();
    expect(fetchSessionById).toHaveBeenCalledWith(
      "sess-001",
      { profile: "default" },
      expect.any(AbortSignal)
    );
  });

  it("reports detail not found without falling back to the full session list", async () => {
    vi.mocked(fetchSessionById).mockRejectedValue(new Error("Session not found: missing-session"));

    const { result } = renderHook(() => useSessionById("missing-session", "ws_alpha"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.error?.message).toBe("Session not found: missing-session");
    });

    expect(fetchSessions).not.toHaveBeenCalled();
    expect(fetchSessionById).toHaveBeenCalledWith(
      "missing-session",
      { profile: "default" },
      expect.any(AbortSignal)
    );
  });
});

describe("session ledger availability projection", () => {
  it.each(["not-materialized", "unsupported"] as const)(
    "Should expose %s independently of the adapter error",
    async reason => {
      vi.mocked(fetchSessionLedger).mockRejectedValue(
        new SessionLedgerUnavailableError("sess-001", reason)
      );
      const { result } = renderHook(() => useSessionLedger("sess-001", "ws_alpha"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.availability).toBe(reason));
      expect(result.current.isLoading).toBe(false);
    }
  );

  it("Should retain unexpected ledger read failures as errors", async () => {
    const error = new Error("ledger materializer crashed");
    vi.mocked(fetchSessionLedger).mockRejectedValue(error);
    const { result } = renderHook(() => useSessionLedger("sess-001", "ws_alpha"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.error).toBe(error), { timeout: 3000 });
    expect(result.current.availability).toBeUndefined();
  });
});

// Invariant: the usage read alone owns context; ledger sequence fences observations while equal-sequence policy and attribution remain live.
// Owner and canonical suite: session query hooks; HTTP responses are supplied at the adapter I/O boundary.

describe("Session context query projection", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });
  it("Should retain sequenced observations, refresh attribution and policy, and survive unavailable reads", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = sessionKeys.usage("ws", "session");
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    vi.mocked(fetchSessionUsage).mockResolvedValue(sessionContextUsageFixture);
    const { result, unmount } = renderHook(() => useSessionContext("session", "ws", "stopped"), {
      wrapper,
    });
    await waitFor(() => expect(result.current.context.ratio).toBe(89_700 / 256_000));
    const update = async (usage: SessionUsagePayload) => {
      await act(async () => {
        client.setQueryData(key, usage);
      });
    };
    await update({
      ...sessionContextUsageFixture,
      context: { ...sessionContextUsageFixture.context, used: 225_280, ratio: 0.88, sequence: 500 },
    });
    await waitFor(() => expect(result.current.context.warning).toBe(true));
    await update({
      ...sessionContextUsageFixture,
      cache_read_tokens: 900,
      context: {
        ...sessionContextUsageFixture.context,
        sequence: 499,
        injected: { estimate: "bytes_div_4", rows: [], tokens: 999, stale: false },
        pressure_threshold: 0.9,
      },
    });
    await waitFor(() => expect(result.current.context.injected?.tokens).toBe(999));
    expect(result.current.context.ratio).toBe(0.88);
    expect(result.current.context.warning).toBe(false);
    expect(result.current.usage?.cache_read_tokens).toBe(900);
    await update({
      ...sessionContextUsageFixture,
      context: { ...sessionContextUsageFixture.context, sequence: 500, pressure_threshold: 0.8 },
    });
    await waitFor(() => expect(result.current.context.warning).toBe(true));
    expect(result.current.context.used).toBe(225_280);
    await update({ ...sessionContextUsageFixture, context: { state: "unavailable" } });
    await waitFor(() => expect(result.current.context.state).toBe("unavailable"));
    expect(result.current.context.used).toBe(225_280);
    vi.mocked(fetchSessionUsage).mockRejectedValue(new Error("offline"));
    await act(async () => {
      await client.invalidateQueries({ queryKey: key, exact: true });
    });
    expect(result.current.context.used).toBe(225_280);
    expect(result.current.context.state).toBe("unavailable");
    unmount();
    client.clear();
  });

  it("Should reset retained observations on the explicit reset signal and use the turns route", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    vi.mocked(fetchSessionUsage).mockResolvedValue(sessionContextUsageFixture);
    vi.mocked(fetchSessionUsageTurns).mockResolvedValue(sessionContextTurnsFixture);
    const { result, unmount, rerender } = renderHook(
      ({ enabled }) => ({
        context: useSessionContext("session", "ws", "stopped", { enabled }),
        turns: useSessionUsageTurns("session", "ws", "stopped"),
      }),
      { wrapper, initialProps: { enabled: false } }
    );
    expect(result.current.context.context.loading).toBe(false);
    rerender({ enabled: true });
    await waitFor(() => expect(result.current.context.context.used).toBe(89_700));
    await waitFor(() => expect(result.current.turns.data).toEqual(sessionContextTurnsFixture));
    vi.mocked(fetchSessionUsage).mockResolvedValue({
      context: { state: "unknown" },
      turn_count: 0,
    });
    await act(async () => {
      await client.resetQueries({ queryKey: sessionKeys.usage("ws", "session"), exact: true });
      client.setQueryData(sessionKeys.contextReset("ws", "session"), 1);
    });
    await waitFor(() => expect(result.current.context.context.state).toBe("unknown"));
    expect(result.current.context.context.used).toBeUndefined();
    expect(sessionUsageOptions("ws", "session", "stopped").refetchInterval).toBe(false);
    expect(sessionUsageTurnsOptions("ws", "session", "stopped").refetchInterval).toBe(false);
    unmount();
    client.clear();
  });
});
