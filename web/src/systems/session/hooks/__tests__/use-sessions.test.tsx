import { primarySessionFixture } from "../../mocks/fixtures";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionPayload } from "../../types";
import { useSessionCatalog } from "../use-session-catalog";
import { useSessionsBoard } from "../use-sessions-board";
import type { DataScope } from "@/systems/workspace";
import { fetchSessionCatalogPage, fetchSessionFacets } from "../../adapters/session-catalog-api";
import { sessionCatalogOptions } from "../../lib/session-catalog-options";
import { sessionKeys } from "../../lib/query-keys";
import { useSession, useSessionById, useSessions } from "../use-sessions";
import { fetchSessions } from "../../adapters/session-api";
import { fetchSessionById } from "../../adapters/session-owner-api";
import { useSessionContext, useSessionUsageTurns } from "../use-session-context";
import { fetchSessionUsage, fetchSessionUsageTurns } from "../../adapters/session-api";
import { sessionUsageOptions, sessionUsageTurnsOptions } from "../../lib/query-options";
import {
  sessionContextFixture,
  sessionContextUsageFixture,
  sessionContextTurnsFixture,
} from "../../mocks/context-fixtures";
import type { SessionUsagePayload } from "../../types";

vi.mock("../../adapters/session-api", () => ({
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
  SessionNotFoundError: class SessionNotFoundError extends Error {
    constructor(public readonly sessionId: string) {
      super(`Session not found: ${sessionId}`);
      this.name = "SessionNotFoundError";
    }
  },
}));

vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
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
    board_column: "done",
    pull_request: null,
    pull_request_lookup: null,
    supervision: null,
    scope: "workspace",
    effective_workspace_access: "",
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
  it("Should suspend shared detail readers until a pending deletion rolls back", async () => {
    vi.mocked(fetchSessionById).mockResolvedValue({ ...primarySessionFixture, id: "sess-001" });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useSession("sess-001"), {
      wrapper: createWrapperWithClient(client),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    let rejectDelete!: (error: Error) => void;
    const deletion = client.getMutationCache().build(client, {
      mutationKey: sessionKeys.deletions(),
      mutationFn: (_target: { id: string }) =>
        new Promise<void>((_resolve, reject) => {
          rejectDelete = reject;
        }),
    });
    let completion!: Promise<unknown>;
    await act(async () => {
      completion = deletion.execute({ id: "sess-001" }).catch(() => undefined);
    });
    await waitFor(() => expect(result.current.isEnabled).toBe(false));
    const count = vi.mocked(fetchSessionById).mock.calls.length;
    await act(async () => {
      await client.invalidateQueries({ queryKey: sessionKeys.byIdRoot("sess-001") });
    });
    expect(fetchSessionById).toHaveBeenCalledTimes(count);
    await act(async () => {
      rejectDelete(new Error("Deletion refused"));
      await completion;
    });
    await waitFor(() => expect(result.current.isEnabled).toBe(true));
    await waitFor(() =>
      expect(vi.mocked(fetchSessionById).mock.calls.length).toBeGreaterThan(count)
    );
  });

  it("Should keep detail readers suspended after a successful deletion", async () => {
    vi.mocked(fetchSessionById).mockResolvedValue({ ...primarySessionFixture, id: "sess-001" });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useSession("sess-001"), {
      wrapper: createWrapperWithClient(client),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    let resolveDelete!: () => void;
    const deletion = client.getMutationCache().build(client, {
      mutationKey: sessionKeys.deletions(),
      mutationFn: (_target: { id: string }) =>
        new Promise<void>(resolve => {
          resolveDelete = resolve;
        }),
    });
    let completion!: Promise<unknown>;
    await act(async () => {
      completion = deletion.execute({ id: "sess-001" }).catch(() => undefined);
    });
    await waitFor(() => expect(result.current.isEnabled).toBe(false));
    const count = vi.mocked(fetchSessionById).mock.calls.length;
    await act(async () => {
      await client.invalidateQueries({ queryKey: sessionKeys.byIdRoot("sess-001") });
    });
    expect(fetchSessionById).toHaveBeenCalledTimes(count);
    await act(async () => {
      resolveDelete();
      await completion;
    });
    expect(result.current.isEnabled).toBe(false);
    await act(async () => {
      await client.invalidateQueries({ queryKey: sessionKeys.byIdRoot("sess-001") });
    });
    expect(fetchSessionById).toHaveBeenCalledTimes(count);
  });

  it("loads a single session detail", async () => {
    vi.mocked(fetchSessionById).mockResolvedValue({
      board_column: "done",
      pull_request: null,
      pull_request_lookup: null,
      supervision: null,
      profile_id: "00000000000000000000000000",
      profile_name: "default",
      scope: "workspace",
      effective_workspace_access: "",
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

// Invariant: the usage read alone owns context; ledger sequence fences observations while equal-sequence attribution remains live.
// Owner and canonical suite: session query hooks; HTTP responses are supplied at the adapter I/O boundary.

describe("Session context query projection", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });
  it("Should retain sequenced observations, refresh attribution, and survive unavailable reads", async () => {
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
    await waitFor(() => expect(result.current.context.ratio).toBe(0.88));
    await update({
      ...sessionContextUsageFixture,
      cache_read_tokens: 900,
      context: {
        ...sessionContextUsageFixture.context,
        sequence: 499,
        injected: { estimate: "bytes_div_4", rows: [], tokens: 999, stale: false },
      },
    });
    await waitFor(() => expect(result.current.context.injected?.tokens).toBe(999));
    expect(result.current.context.ratio).toBe(0.88);
    expect(result.current.usage?.cache_read_tokens).toBe(900);
    await update({
      ...sessionContextUsageFixture,
      context: { ...sessionContextUsageFixture.context, sequence: 500 },
    });
    await waitFor(() => expect(result.current.context.injected?.tokens).toBe(12_400));
    expect(result.current.context.used).toBe(225_280);
    await update({
      ...sessionContextUsageFixture,
      input_tokens: 12345,
      cache_read_tokens: 9876,
      context: { state: "unavailable" },
    });
    await waitFor(() => expect(result.current.context.state).toBe("unavailable"));
    expect(result.current.usage?.input_tokens).toBe(12345);
    expect(result.current.usage?.cache_read_tokens).toBe(9876);
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

  it("Should clear the reading at a terminal compaction until the agent's next report", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = sessionKeys.usage("ws", "session");
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    vi.mocked(fetchSessionUsage).mockResolvedValue(sessionContextUsageFixture);
    const { result, unmount } = renderHook(() => useSessionContext("session", "ws", "stopped"), {
      wrapper,
    });
    await waitFor(() => expect(result.current.context.used).toBe(89_700));
    const update = async (usage: SessionUsagePayload) => {
      await act(async () => {
        client.setQueryData(key, usage);
      });
    };
    // The daemon's answer after the compaction boundary: occupancy unknown, attribution kept.
    await update({
      ...sessionContextUsageFixture,
      context: {
        state: "unknown",
        used: null,
        size: null,
        ratio: null,
        injected: { ...sessionContextFixture.injected!, tokens: 999 },
      },
    });
    await waitFor(() => expect(result.current.context.state).toBe("unknown"));
    expect(result.current.context.used).toBeNull();
    expect(result.current.context.sequence).toBeUndefined();
    expect(result.current.context.injected?.tokens).toBe(999);
    // A later failed read keeps the cleared reading rather than restoring the old one.
    await update({ ...sessionContextUsageFixture, context: { state: "unavailable" } });
    await waitFor(() => expect(result.current.context.state).toBe("unavailable"));
    expect(result.current.context.used).toBeNull();
    // A late response from before the boundary is stale: it refreshes attribution, not the ratio.
    await update({
      ...sessionContextUsageFixture,
      cache_read_tokens: 4321,
      context: { ...sessionContextUsageFixture.context, sequence: 412 },
    });
    await waitFor(() => expect(result.current.usage?.cache_read_tokens).toBe(4321));
    expect(result.current.context.state).toBe("unknown");
    expect(result.current.context.used).toBeNull();
    // The next usage report after the boundary is the first reading shown again.
    await update({
      ...sessionContextUsageFixture,
      context: { ...sessionContextUsageFixture.context, used: 31_000, ratio: 0.12, sequence: 520 },
    });
    await waitFor(() => expect(result.current.context.used).toBe(31_000));
    expect(result.current.context.state).toBe("reported");
    // After a fresh report the usual sequence rule governs again: an older read cannot regress it.
    await update({
      ...sessionContextUsageFixture,
      context: { ...sessionContextUsageFixture.context, sequence: 519 },
    });
    await waitFor(() => expect(result.current.context.used).toBe(31_000));
    unmount();
    client.clear();
  });

  it("Should refuse a report from before the boundary the daemon names, even on a fresh load", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = sessionKeys.usage("ws", "session");
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    // First load of a window opened after the compaction: no reading was ever retained.
    vi.mocked(fetchSessionUsage).mockResolvedValue({
      ...sessionContextUsageFixture,
      context: {
        state: "unknown",
        used: null,
        size: null,
        ratio: null,
        cleared_by: { compaction_id: "compaction-1", sequence: 600 },
      },
    });
    const { result, unmount } = renderHook(() => useSessionContext("session", "ws", "stopped"), {
      wrapper,
    });
    await waitFor(() =>
      expect(result.current.context.cleared_by).toEqual({
        compaction_id: "compaction-1",
        sequence: 600,
      })
    );
    expect(result.current.context.state).toBe("unknown");
    const update = async (usage: SessionUsagePayload) => {
      await act(async () => {
        client.setQueryData(key, usage);
      });
    };
    // A late response taken before the named boundary refreshes attribution only.
    await update({
      ...sessionContextUsageFixture,
      cache_read_tokens: 777,
      context: { ...sessionContextUsageFixture.context, sequence: 600 },
    });
    await waitFor(() => expect(result.current.usage?.cache_read_tokens).toBe(777));
    expect(result.current.context.state).toBe("unknown");
    expect(result.current.context.used).toBeNull();
    // The first report after the boundary restores the reading and drops the cause.
    await update({
      ...sessionContextUsageFixture,
      context: { ...sessionContextUsageFixture.context, used: 31_000, ratio: 0.12, sequence: 601 },
    });
    await waitFor(() => expect(result.current.context.used).toBe(31_000));
    expect(result.current.context.cleared_by).toBeUndefined();
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

const ALPHA: DataScope = { kind: "project", workspaceId: "ws_alpha" };
const BETA: DataScope = { kind: "project", workspaceId: "ws_beta" };

describe("bounded session catalog", () => {
  beforeEach(() => {
    vi.mocked(fetchSessionCatalogPage).mockReset();
    vi.mocked(fetchSessionFacets).mockReset();
  });
  it("Should share exact population facets across differently filtered visible pages", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: {
        terminal_approvals: 0,
        all: 201,
        needs_you: 1,
        working: 0,
        finished: 0,
        idle: 200,
        board: { needs_you: 0, working: 0, in_review: 0, done: 0 },
      },
      by_workspace: [],
      by_profile: [],
    });
    vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 100 },
    });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    const { result } = renderHook(
      () => ({
        search: useSessionCatalog(
          ALPHA,
          {
            q: "older",
            search_fields: "title_agent",
            sort: "last_activity",
            limit: 100,
            parent: " parent-session ",
            root: "root-session",
          },
          true,
          { facets: true }
        ),
        attention: useSessionCatalog(
          ALPHA,
          {
            attention: true,
            sort: "attention",
            include_health: true,
            limit: 50,
            parent: "parent-session",
            root: "root-session",
          },
          true,
          { facets: true }
        ),
        picker: useSessionCatalog(ALPHA, { q: "picker", limit: 100 }, true, { facets: false }),
      }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.search.total).toBe(201));
    expect(result.current.attention.total).toBe(201);
    expect(fetchSessionFacets).toHaveBeenCalledTimes(1);
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      {
        workspace_id: "ws_alpha",
        profile: "default",
        parent: "parent-session",
        root: "root-session",
      },
      expect.any(AbortSignal)
    );
  });

  it("Should keep one visible page on invalidation and navigate older history explicitly", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: {
        terminal_approvals: 0,
        all: 201,
        needs_you: 0,
        working: 0,
        finished: 0,
        idle: 201,
        board: { needs_you: 0, working: 0, in_review: 0, done: 0 },
      },
      by_workspace: [],
      by_profile: [],
    });
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async query => ({
      sessions: [makeSession({ id: query?.cursor ? "sess-old" : "sess-new" })],
      page: {
        has_more: !query?.cursor,
        next_cursor: query?.cursor ? undefined : "older",
        limit: 100,
      },
    }));
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    const { result } = renderHook(
      () => useSessionCatalog(ALPHA, { limit: 100 }, true, { facets: true }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.sessions[0]?.id).toBe("sess-new"));
    expect(result.current.total).toBe(201);
    expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(1);
    act(() => result.current.nextPage());
    await waitFor(() => expect(result.current.sessions[0]?.id).toBe("sess-old"));
    vi.mocked(fetchSessionCatalogPage).mockClear();
    await act(async () => {
      await client.invalidateQueries({ queryKey: sessionKeys.workspaceLists("ws_alpha") });
    });
    expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(1);
    expect(fetchSessionCatalogPage).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: "older", skip_total: true }),
      expect.any(AbortSignal)
    );
    expect(
      client.getQueryData<{ pages: unknown[] }>(
        sessionCatalogOptions({ workspace_id: "ws_alpha", limit: 100, profile: "default" }).queryKey
      )?.pages
    ).toHaveLength(1);
    act(() => result.current.previousPage());
    await waitFor(() => expect(result.current.sessions[0]?.id).toBe("sess-new"));
  });
  it("Should retain search rows without reusing another workspace or fetching unused facets", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionCatalogPage).mockResolvedValueOnce({
      sessions: [makeSession({ id: "sess-original" })],
      page: { has_more: true, next_cursor: "old-cursor", limit: 100 },
    });
    // Invariant: a superseded catalog read is cancellation, never failed UI state.
    // Owner: TanStack catalog hook; canonical suite: this existing search/scope test.
    const cancelled = vi.fn();
    vi.mocked(fetchSessionCatalogPage).mockImplementation(
      (_filters, signal) =>
        new Promise((_resolve, reject) => {
          signal?.addEventListener("abort", () => {
            cancelled();
            reject(new DOMException("Superseded", "AbortError"));
          });
        })
    );
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    const { result, rerender, unmount } = renderHook(
      ({ workspace, search }) => useSessionCatalog(workspace, { q: search, limit: 100 }),
      { wrapper, initialProps: { workspace: ALPHA, search: "original" } }
    );
    await waitFor(() => expect(result.current.sessions[0]?.id).toBe("sess-original"));
    expect(fetchSessionFacets).not.toHaveBeenCalled();
    rerender({ workspace: ALPHA, search: "changed" });
    expect(result.current.sessions[0]?.id).toBe("sess-original");
    expect(result.current.paging).toBe(true);
    expect(result.current.next).toBe(false);
    expect(result.current.previous).toBe(false);
    act(() => result.current.nextPage());
    expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(2);
    rerender({ workspace: BETA, search: "changed" });
    expect(result.current.sessions).toEqual([]);
    expect(result.current.loading).toBe(true);
    expect(result.current.failed).toBe(false);
    expect(cancelled).toHaveBeenCalledTimes(1);
    expect(fetchSessionFacets).not.toHaveBeenCalled();
    unmount();
    client.clear();
  });
});

// Invariant (SK-FR-007): only a board read with nothing loaded fails the whole column; a
// failed next page or background re-read keeps the loaded cards and retries that read.
// Owner: the board read model; canonical suite: this catalog query file.
describe("sessions board read failures", () => {
  const FACETS = {
    facets: {
      terminal_approvals: 0,
      all: 300,
      needs_you: 0,
      working: 0,
      finished: 0,
      idle: 300,
      board: { needs_you: 0, working: 0, in_review: 0, done: 300 },
    },
    by_workspace: [],
    by_profile: [],
  };

  beforeEach(() => {
    vi.mocked(fetchSessionCatalogPage).mockReset();
    vi.mocked(fetchSessionFacets).mockReset();
    vi.mocked(fetchSessionFacets).mockResolvedValue(FACETS);
  });

  function renderBoard(client: QueryClient) {
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    return renderHook(
      () =>
        useSessionsBoard({ scope: ALPHA, hideChildren: false, sort: "created", groupBy: "none" }),
      { wrapper }
    );
  }

  it("Should keep loaded cards when the next page or a re-read fails", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    let failNext = true;
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async query => {
      if (!query?.board_column?.includes("done")) {
        return { sessions: [], page: { has_more: false, limit: 50 } };
      }
      if (query.cursor && failNext) throw new Error("offline");
      return {
        sessions: [makeSession({ id: query.cursor ? "sess-page-2" : "sess-page-1" })],
        page: { has_more: !query.cursor, next_cursor: query.cursor ? undefined : "p2", limit: 50 },
      };
    });
    const { result } = renderBoard(client);
    const done = () => result.current.columns.find(column => column.id === "done")!;
    await waitFor(() =>
      expect(done().sessions.map(session => session.id)).toEqual(["sess-page-1"])
    );

    act(() => done().loadMore());
    await waitFor(() => expect(done().failure).toBe("more"));
    expect(done().sessions.map(session => session.id)).toEqual(["sess-page-1"]);

    failNext = false;
    act(() => done().retry());
    await waitFor(() =>
      expect(done().sessions.map(session => session.id)).toEqual(["sess-page-1", "sess-page-2"])
    );
    expect(done().failure).toBeNull();

    vi.mocked(fetchSessionCatalogPage).mockRejectedValue(new Error("offline"));
    await act(async () => {
      await client.refetchQueries({ queryKey: sessionKeys.workspaceLists("ws_alpha") });
    });
    await waitFor(() => expect(done().failure).toBe("refresh"));
    expect(done().sessions.map(session => session.id)).toEqual(["sess-page-1", "sess-page-2"]);
    client.clear();
  });

  it("Should fail a column only when its first read fails", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async query => {
      if (query?.board_column?.includes("in_review")) throw new Error("offline");
      return { sessions: [], page: { has_more: false, limit: 50, total: 0 } };
    });
    const { result } = renderBoard(client);
    const column = (id: string) => result.current.columns.find(item => item.id === id)!;
    await waitFor(() => expect(column("in_review").failure).toBe("initial"));
    expect(column("working").failure).toBeNull();
    expect(result.current.archive.count).toBe(0);
    client.clear();
  });

  // Invariant (SK-FR-008): a failed archive read is unknown, never a confirmed zero.
  it("Should leave the archive count unknown when its first read fails", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async query => {
      if (query?.archive === "only") throw new Error("offline");
      return { sessions: [], page: { has_more: false, limit: 50 } };
    });
    const { result } = renderBoard(client);
    await waitFor(() => expect(result.current.archive.failure).toBe("initial"));
    expect(result.current.archive.count).toBeUndefined();
    client.clear();
  });
});

// Invariant (#952): board columns read in the window's sort, a List-only `manual` reading
// `last_activity`; grouped by project, the board lists one lane per facet group (never derived
// from a loaded page), counted with the same population filters as its columns, and leaves the
// ungrouped columns unread. Owner: the board read model.
describe("sessions board order and swimlanes", () => {
  const facet = (all: number) => ({
    terminal_approvals: 0,
    all,
    needs_you: 0,
    working: all,
    finished: 0,
    idle: 0,
    board: { needs_you: 0, working: all, in_review: 0, done: 0 },
  });

  beforeEach(() => {
    vi.mocked(fetchSessionCatalogPage).mockReset();
    vi.mocked(fetchSessionFacets).mockReset();
    vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 50 },
    });
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: facet(3),
      by_workspace: [
        { workspace_id: "ws_alpha", facets: facet(2) },
        { workspace_id: "ws_beta", facets: facet(1) },
        { workspace_id: "ws_empty", facets: facet(0) },
      ],
      by_profile: [],
    });
  });

  function renderBoard(input: Parameters<typeof useSessionsBoard>[0]) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    return { client, ...renderHook(() => useSessionsBoard(input), { wrapper }) };
  }

  it("Should read every column in last-activity order while the stored sort is manual", async () => {
    const { client } = renderBoard({
      scope: ALPHA,
      hideChildren: false,
      sort: "manual",
      groupBy: "none",
    });
    await waitFor(() => expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(5));
    const sorts = vi.mocked(fetchSessionCatalogPage).mock.calls.map(([query]) => query?.sort);
    expect(new Set(sorts)).toEqual(new Set(["last_activity"]));
    client.clear();
  });

  it("Should list one lane per project with sessions and leave the flat columns unread", async () => {
    const { result, client } = renderBoard({
      scope: { kind: "all" },
      hideChildren: true,
      sort: "name",
      groupBy: "project",
    });
    await waitFor(() => expect(result.current.lanes?.length).toBe(2));
    expect(result.current.groupBy).toBe("project");
    expect(result.current.columns).toEqual([]);
    expect(result.current.lanes!.map(lane => lane.filters)).toEqual([
      expect.objectContaining({ workspace_id: "ws_alpha", children: "exclude", sort: "name" }),
      expect.objectContaining({ workspace_id: "ws_beta", children: "exclude", sort: "name" }),
    ]);
    // Lane membership counts the rows the lanes' columns show: every subagent, children hidden.
    const membership = vi
      .mocked(fetchSessionFacets)
      .mock.calls.map(([query]) => query)
      .find(query => query?.all_workspaces);
    expect(membership).toMatchObject({ children: "exclude" });
    expect(membership).not.toHaveProperty("subagents");
    // Only the Archive bar reads a page here: each lane reads its own columns when it renders.
    const reads = vi.mocked(fetchSessionCatalogPage).mock.calls.map(([query]) => query);
    expect(reads.every(query => query?.archive === "only")).toBe(true);
    client.clear();
  });

  it("Should render ungrouped where the stored grouping does not apply", async () => {
    const { result, client } = renderBoard({
      scope: ALPHA,
      hideChildren: false,
      sort: "created",
      groupBy: "project",
    });
    await waitFor(() => expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(5));
    expect(result.current.groupBy).toBe("none");
    expect(result.current.lanes).toBeNull();
    expect(result.current.columns).toHaveLength(4);
    client.clear();
  });
});

// Invariant (W-06): the User destination lists through `@user`, never the aggregate, and an
// unsettled scope reads nothing. Owner: the catalog hook every session list goes through.
describe("session catalog data scope", () => {
  beforeEach(() => {
    vi.mocked(fetchSessionCatalogPage).mockReset();
    vi.mocked(fetchSessionFacets).mockReset();
    vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 100 },
    });
  });

  function wrapperFor(client: QueryClient) {
    return ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
  }

  it("Should request User sessions through @user, never all_workspaces", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderHook(() => useSessionCatalog({ kind: "user" }, { limit: 100 }), {
      wrapper: wrapperFor(client),
    });
    await waitFor(() => expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(1));
    const [filters] = vi.mocked(fetchSessionCatalogPage).mock.calls[0]!;
    expect(filters).toMatchObject({ workspace_id: "@user" });
    expect(filters).not.toHaveProperty("all_workspaces");
    client.clear();
  });

  it("Should request every project only under All projects", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderHook(() => useSessionCatalog({ kind: "all" }, { limit: 100 }), {
      wrapper: wrapperFor(client),
    });
    await waitFor(() => expect(fetchSessionCatalogPage).toHaveBeenCalledTimes(1));
    const [filters] = vi.mocked(fetchSessionCatalogPage).mock.calls[0]!;
    expect(filters).toMatchObject({ all_workspaces: true });
    expect(filters).not.toHaveProperty("workspace_id");
    client.clear();
  });

  it("Should read nothing while the scope is unsettled", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(
      () => useSessionCatalog(null, { limit: 100 }, true, { facets: true }),
      { wrapper: wrapperFor(client) }
    );
    await act(async () => {
      await Promise.resolve();
    });
    expect(fetchSessionCatalogPage).not.toHaveBeenCalled();
    expect(fetchSessionFacets).not.toHaveBeenCalled();
    expect(result.current.sessions).toEqual([]);
    client.clear();
  });
});
