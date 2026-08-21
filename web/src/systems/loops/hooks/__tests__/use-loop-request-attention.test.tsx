// Suite: Loop request attention projections
// Invariant: each workspace contributes its daemon aggregate, while run indicators group every
// pending workspace page; a failed source contributes zero without hiding healthy sources.
// Owning layer: loop request attention query composition.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { pendingAskRequest, pendingReviewRequest } from "../../mocks/fixture-graph-eng-requests";

const useQueriesMock = vi.hoisted(() => vi.fn());
const useQueryMock = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return { ...actual, useQueries: useQueriesMock, useQuery: useQueryMock };
});

const { useLoopRequestAttention } = await import("../use-loop-request-attention");

function result(
  pending: number,
  items = [pendingAskRequest],
  isError = false
): Record<string, unknown> {
  return {
    data: { aggregates: { pending }, items, next_cursor: "" },
    isError,
    isLoading: false,
  };
}

describe("useLoopRequestAttention", () => {
  beforeEach(() => useQueriesMock.mockReset());

  it("Should sum exact pending aggregates across healthy workspaces", () => {
    useQueriesMock.mockReturnValue([
      result(4, [pendingAskRequest]),
      result(2, [pendingReviewRequest]),
    ]);

    const { result: hook } = renderHook(() =>
      useLoopRequestAttention([
        { id: "ws-a", name: "alpha" },
        { id: "ws-b", name: "beta" },
      ])
    );

    expect(hook.current.pendingCount).toBe(6);
    expect(hook.current.items).toEqual([
      expect.objectContaining({ workspaceId: "ws-a", workspaceLabel: "alpha", stale: false }),
      expect.objectContaining({ workspaceId: "ws-b", workspaceLabel: "beta", stale: false }),
    ]);
    expect(hook.current.disconnected).toBe(false);
  });

  it("Should preserve healthy workspaces while an errored workspace contributes zero", () => {
    useQueriesMock.mockReturnValue([
      result(4, [pendingAskRequest]),
      result(9, [pendingReviewRequest], true),
    ]);

    const { result: hook } = renderHook(() =>
      useLoopRequestAttention([
        { id: "ws-a", name: "alpha" },
        { id: "ws-b", name: "beta" },
      ])
    );

    expect(hook.current.pendingCount).toBe(4);
    expect(hook.current.items[0]).toMatchObject({ workspaceId: "ws-a", stale: false });
    expect(hook.current.items[1]).toMatchObject({ workspaceId: "ws-b", stale: true });
    expect(hook.current.disconnected).toBe(true);
  });
});
