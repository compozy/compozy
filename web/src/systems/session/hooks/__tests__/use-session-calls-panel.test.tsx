// Invariant: a retained call loses only its counterpart jump, and only after a
// complete, successful root-catalog read proves that counterpart was pruned.
// Owning layer: useSessionCallsPanel. Canonical suite: this hook test; the
// inspector component suite owns presentation only.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { buildCallFixture, completedCallFixture } from "@/systems/agent-comms/mocks";

import { useSessionCallsPanel } from "../use-session-calls-panel";

const { catalogRef, sessionRef, madeCallsRef, receivedCallsRef, querySpies, optionSpies } =
  vi.hoisted(() => ({
    catalogRef: {
      current: {
        data: undefined as
          | { sessions: Array<{ id: string; name?: string; agent_name?: string }> }
          | undefined,
        isSuccess: false,
        isError: false,
        isPending: false,
      },
    },
    sessionRef: {
      current: {
        data: undefined as { lineage?: { root_session_id?: string } } | undefined,
      },
    },
    madeCallsRef: { current: [] as unknown[] },
    receivedCallsRef: { current: [] as unknown[] },
    querySpies: {
      madeLoadMore: vi.fn(),
      madeRetry: vi.fn(),
      receivedLoadMore: vi.fn(),
      receivedRetry: vi.fn(),
    },
    optionSpies: {
      list: vi.fn(
        (
          _scope: unknown,
          filter: { caller?: string; child_session_id?: string },
          live: boolean,
          enabled: boolean
        ) => ({ ...filter, live, enabled })
      ),
      count: vi.fn(
        (_scope: unknown, _filter: unknown, _options: { enabled: boolean; live: boolean }) => 0
      ),
      complete: vi.fn((filters: unknown) => ({ complete: true, filters })),
    },
  }));

vi.mock("@tanstack/react-query", () => ({
  useInfiniteQuery: (filter: { caller?: string }) => {
    const made = Boolean(filter.caller);
    return {
      data: { pages: [{ items: made ? madeCallsRef.current : receivedCallsRef.current }] },
      hasNextPage: made,
      isFetchingNextPage: made,
      isError: false,
      isFetchNextPageError: false,
      error: null,
      fetchNextPage: made ? querySpies.madeLoadMore : querySpies.receivedLoadMore,
      refetch: made ? querySpies.madeRetry : querySpies.receivedRetry,
    };
  },
  useQuery: (options: { complete?: boolean }) => {
    if (options.complete) return catalogRef.current;
    return sessionRef.current;
  },
}));

vi.mock("@/systems/agent-comms", () => ({
  CALLS_PANEL_PAGE_SIZE: 25,
  callsListOptions: optionSpies.list,
  useAgentCommsScope: () => ({
    workspaceId: "ws_main",
    profileKey: "profile:default",
    actingProfile: "default",
    params: { profile: "default" },
    profileScope: { profile: "default" },
  }),
  useCallCount: (
    scope: unknown,
    filter: { caller?: string },
    options: { enabled: boolean; live: boolean }
  ) => {
    optionSpies.count(scope, filter, options);
    return filter.caller ? 11 : 7;
  },
}));

vi.mock("../../lib/query-options", () => ({
  sessionsCompleteListOptions: (filters: unknown) => optionSpies.complete(filters),
}));

vi.mock("../use-sessions", () => ({
  useSession: () => sessionRef.current,
}));

describe("useSessionCallsPanel — retained counterpart availability", () => {
  const rootSessionId = completedCallFixture.root_session_id;
  const childSessionId = completedCallFixture.child_session_id!;
  const callerSessionId = "ses_pruned_caller";

  beforeEach(() => {
    vi.clearAllMocks();
    madeCallsRef.current = [completedCallFixture];
    receivedCallsRef.current = [
      buildCallFixture({
        call_id: "call_received",
        caller: { id: callerSessionId, kind: "session" },
        child_session_id: rootSessionId,
        root_session_id: rootSessionId,
      }),
    ];
    sessionRef.current = {
      data: { lineage: { root_session_id: rootSessionId } },
    };
    catalogRef.current = {
      data: { sessions: [{ id: rootSessionId }] },
      isSuccess: true,
      isError: false,
      isPending: false,
    };
  });

  it("Should keep each direction's rows, totals, pagination, and callbacks separate", () => {
    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));

    expect(result.current.made).toMatchObject({
      calls: [completedCallFixture],
      total: 11,
      hasMore: true,
      loadingMore: true,
      error: null,
    });
    expect(result.current.received).toMatchObject({
      calls: [expect.objectContaining({ call_id: "call_received" })],
      total: 7,
      hasMore: false,
      loadingMore: false,
      error: null,
    });

    result.current.made.onLoadMore?.();
    result.current.made.onRetry?.();
    result.current.received.onLoadMore?.();
    result.current.received.onRetry?.();

    expect(querySpies.madeLoadMore).toHaveBeenCalledOnce();
    expect(querySpies.madeRetry).toHaveBeenCalledOnce();
    expect(querySpies.receivedLoadMore).toHaveBeenCalledOnce();
    expect(querySpies.receivedRetry).toHaveBeenCalledOnce();
  });

  it("Should disable list and count reads when retained-window live data is disabled", () => {
    renderHook(() => useSessionCallsPanel(rootSessionId, false));

    expect(optionSpies.list).toHaveBeenCalledTimes(2);
    expect(optionSpies.list).toHaveBeenNthCalledWith(
      1,
      expect.anything(),
      expect.objectContaining({ caller: rootSessionId }),
      false,
      false
    );
    expect(optionSpies.list).toHaveBeenNthCalledWith(
      2,
      expect.anything(),
      expect.objectContaining({ child_session_id: rootSessionId }),
      false,
      false
    );
    expect(optionSpies.count).toHaveBeenCalledTimes(2);
    expect(optionSpies.count).toHaveBeenCalledWith(expect.anything(), expect.anything(), {
      enabled: false,
      live: false,
    });
  });

  it("Should mark missing made and received counterparts after the catalog is complete", () => {
    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));

    expect(result.current.prunedSessionIds).toEqual(new Set([childSessionId, callerSessionId]));
  });

  it("Should resolve caller labels from daemon-shaped session catalog identities", () => {
    catalogRef.current = {
      data: {
        sessions: [
          { id: rootSessionId },
          { id: callerSessionId, name: "reviewer", agent_name: "reviewer" },
        ],
      },
      isSuccess: true,
      isError: false,
      isPending: false,
    };

    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));

    expect(result.current.callerNames.get(callerSessionId)).toBe("reviewer");
  });

  it("Should fail open while the catalog is pending", () => {
    catalogRef.current = { data: undefined, isSuccess: false, isError: false, isPending: true };

    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));
    expect(result.current.prunedSessionIds).toEqual(new Set());
  });

  it("Should fail open when the catalog read errors", () => {
    catalogRef.current = { data: undefined, isSuccess: false, isError: true, isPending: false };

    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));
    expect(result.current.prunedSessionIds).toEqual(new Set());
  });

  it("Should pin the counterpart catalog to the session lineage root and workspace", () => {
    renderHook(() => useSessionCallsPanel(rootSessionId));

    expect(optionSpies.complete).toHaveBeenCalledWith({
      workspace_id: "ws_main",
      root: rootSessionId,
      profile: "default",
    });
  });
});
