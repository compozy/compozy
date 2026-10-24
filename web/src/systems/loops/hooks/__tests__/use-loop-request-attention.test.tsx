// Suite: Loop request attention projections
// Invariant: the loops badge counts what the Loops listing would show — one pending-request read
// over the breadth (a project, User, or the All projects aggregate) within the profile lens. The
// count is the daemon's aggregate; a failed read counts zero and reports itself disconnected; each
// item names its own project (empty and "User" for a User-scope run).
// Owning layer: loop request attention query composition.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { pendingAskRequest, pendingReviewRequest } from "../../mocks/fixture-graph-eng-requests";

const useQueryMock = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return { ...actual, useQuery: useQueryMock };
});

vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useProfileReadScope: () => ({ params: { profile: "engineering" } }),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ registeredWorkspaces: [{ id: "ws-a", name: "alpha" }] }),
  useDataScope: () => null,
}));

const { useLoopRequestAttention } = await import("../use-loop-request-attention");

function result(pending: number, items = [pendingAskRequest], isError = false) {
  return { data: { aggregates: { pending }, items, next_cursor: "" }, isError, isLoading: false };
}

describe("useLoopRequestAttention", () => {
  beforeEach(() => useQueryMock.mockReset());

  it("Should read the All projects aggregate once within the profile lens", () => {
    useQueryMock.mockReturnValue(
      result(6, [
        { ...pendingAskRequest, workspace_id: "ws-a" },
        { ...pendingReviewRequest, workspace_id: "" },
      ])
    );

    const { result: hook } = renderHook(() => useLoopRequestAttention({ kind: "all" }));

    expect(useQueryMock).toHaveBeenCalledTimes(1);
    const options = useQueryMock.mock.calls[0]?.[0];
    expect(options.queryKey).toEqual(["loops", "requests", "@all", "attention", "", "engineering"]);
    expect(options.enabled).toBe(true);
    expect(hook.current.pendingCount).toBe(6);
    expect(hook.current.items).toEqual([
      expect.objectContaining({ workspaceId: "ws-a", workspaceLabel: "alpha", stale: false }),
      expect.objectContaining({ workspaceId: "", workspaceLabel: "User", stale: false }),
    ]);
    expect(hook.current.disconnected).toBe(false);
  });

  it("Should read only the focused project and attribute its rows to it", () => {
    useQueryMock.mockReturnValue(result(2, [{ ...pendingAskRequest, workspace_id: undefined }]));

    const { result: hook } = renderHook(() =>
      useLoopRequestAttention({ kind: "project", workspaceId: "ws-a" })
    );

    expect(useQueryMock.mock.calls[0]?.[0].queryKey.slice(0, 4)).toEqual([
      "loops",
      "requests",
      "ws-a",
      "attention",
    ]);
    expect(hook.current.items[0]).toMatchObject({ workspaceId: "ws-a", workspaceLabel: "alpha" });
  });

  it("Should count nothing and report disconnected when the read fails", () => {
    useQueryMock.mockReturnValue(result(9, [pendingReviewRequest], true));

    const { result: hook } = renderHook(() => useLoopRequestAttention({ kind: "user" }));

    expect(useQueryMock.mock.calls[0]?.[0].queryKey[2]).toBe("@user");
    expect(hook.current.pendingCount).toBe(0);
    expect(hook.current.items[0]).toMatchObject({ workspaceId: "", stale: true });
    expect(hook.current.disconnected).toBe(true);
  });

  it("Should stay idle while the data scope is unsettled", () => {
    useQueryMock.mockReturnValue({ data: undefined, isError: false, isLoading: false });

    renderHook(() => useLoopRequestAttention(null));

    expect(useQueryMock.mock.calls[0]?.[0].enabled).toBe(false);
  });
});
