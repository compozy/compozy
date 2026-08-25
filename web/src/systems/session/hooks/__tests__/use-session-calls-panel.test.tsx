// Invariant: a retained call loses only its counterpart jump, and only after a
// complete, successful root-catalog read proves that counterpart was pruned.
// Owning layer: useSessionCallsPanel. Canonical suite: this hook test; the
// inspector component suite owns presentation only.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { buildCallFixture, completedCallFixture } from "@/systems/agent-comms/mocks";

import { useSessionCallsPanel } from "../use-session-calls-panel";

const { catalogRef, madeCallsRef, receivedCallsRef } = vi.hoisted(() => ({
  catalogRef: {
    current: {
      data: undefined as { id: string }[] | undefined,
      hasNextPage: false,
      isError: false,
    },
  },
  madeCallsRef: { current: [] as unknown[] },
  receivedCallsRef: { current: [] as unknown[] },
}));

vi.mock("@tanstack/react-query", () => ({
  useInfiniteQuery: (filter: { caller?: string }) => ({
    data: {
      pages: [{ items: filter.caller ? madeCallsRef.current : receivedCallsRef.current }],
    },
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));

vi.mock("@/systems/agent-comms", () => ({
  CALLS_PANEL_PAGE_SIZE: 25,
  callsListOptions: (_scope: unknown, filter: { caller?: string; child_session_id?: string }) =>
    filter,
  useAgentCommsScope: () => ({
    workspaceId: "ws_main",
    profileKey: "profile:default",
    actingProfile: "default",
    profileScope: { profile: "default" },
  }),
  useCallCount: (_scope: unknown, filter: { caller?: string }) => (filter.caller ? 1 : 1),
}));

vi.mock("../use-sessions", () => ({
  useSessions: () => catalogRef.current,
}));

describe("useSessionCallsPanel — retained counterpart availability", () => {
  const rootSessionId = completedCallFixture.root_session_id;
  const childSessionId = completedCallFixture.child_session_id!;
  const callerSessionId = "ses_pruned_caller";

  beforeEach(() => {
    madeCallsRef.current = [completedCallFixture];
    receivedCallsRef.current = [
      buildCallFixture({
        call_id: "call_received",
        caller: { id: callerSessionId, kind: "session" },
        child_session_id: rootSessionId,
        root_session_id: rootSessionId,
      }),
    ];
    catalogRef.current = {
      data: [{ id: rootSessionId }],
      hasNextPage: false,
      isError: false,
    };
  });

  it("Should mark missing made and received counterparts after the catalog is complete", () => {
    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));

    expect(result.current.prunedSessionIds).toEqual(new Set([childSessionId, callerSessionId]));
  });

  it("Should fail open while the catalog is incomplete", () => {
    catalogRef.current = { data: [{ id: rootSessionId }], hasNextPage: true, isError: false };

    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));
    expect(result.current.prunedSessionIds).toEqual(new Set());
  });

  it("Should fail open when the catalog read errors", () => {
    catalogRef.current = { data: undefined, hasNextPage: false, isError: true };

    const { result } = renderHook(() => useSessionCallsPanel(rootSessionId));
    expect(result.current.prunedSessionIds).toEqual(new Set());
  });
});
