import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import { sessionSelectionCounts, useSessionSelection } from "../use-session-selection";

// Invariant: selection belongs to one list identity and ranges use only rendered order.
// Owner: useSessionSelection; no existing hook suite owns this interaction state.
describe("useSessionSelection", () => {
  it("toggles an ordered set and leaves selection mode on the last uncheck", () => {
    const { result } = renderHook(() => useSessionSelection("workspace", false));
    act(() => result.current.toggle("b"));
    act(() => result.current.toggle("a"));
    expect(result.current.selectedIds).toEqual(["b", "a"]);
    act(() => result.current.toggle("b"));
    act(() => result.current.toggle("a"));
    expect(result.current.mode).toBe(false);
  });

  it("extends from the anchor across visible roots and children in either direction", () => {
    const { result } = renderHook(() => useSessionSelection("workspace", false));
    act(() => result.current.toggle("root-2"));
    act(() => result.current.toggleRange("child", ["root-1", "child", "root-2"]));
    expect(result.current.selectedIds).toEqual(["root-2", "child"]);
    expect(result.current.anchorId).toBe("root-2");
    act(() => result.current.toggleRange("root-1", ["root-1", "root-2"]));
    expect(result.current.selectedIds).toEqual(["root-2", "child", "root-1"]);
    act(() => result.current.toggleRange("absent", ["root-1", "root-2"]));
    expect(result.current.selectedIds).toHaveLength(3);
  });

  it("selects visible rows without losing hidden selection, prunes removed ids, and clears", () => {
    const { result } = renderHook(() => useSessionSelection("workspace", false));
    act(() => result.current.toggle("hidden"));
    act(() => result.current.selectAll(["a", "b", "a"]));
    expect(result.current.selectedIds).toEqual(["hidden", "a", "b"]);
    act(() => result.current.prune(["a", "b"]));
    expect(result.current.selectedIds).toEqual(["a", "b"]);
    expect(result.current.anchorId).toBeNull();
    act(() => result.current.clear());
    expect(result.current.selectedIds).toEqual([]);
  });

  it("clears on scope or archive changes and isolates list instances", () => {
    const { result, rerender } = renderHook(
      ({ scope, archived }) => useSessionSelection(scope, archived),
      { initialProps: { scope: "workspace", archived: false } }
    );
    const other = renderHook(() => useSessionSelection("workspace", false));
    act(() => result.current.toggle("a"));
    rerender({ scope: "workspace", archived: false });
    expect(result.current.mode).toBe(true);
    expect(other.result.current.mode).toBe(false);
    rerender({ scope: "all-workspaces", archived: false });
    expect(result.current.mode).toBe(false);
    act(() => result.current.toggle("a"));
    rerender({ scope: "all-workspaces", archived: true });
    expect(result.current.mode).toBe(false);
  });

  it("derives eligible subsets from runtime state and counts hidden selections", () => {
    const sessions = [
      { ...primarySessionFixture, id: "active", state: "active" as const, archived_at: null },
      { ...primarySessionFixture, id: "starting", state: "starting" as const, archived_at: null },
      { ...primarySessionFixture, id: "stopped", state: "stopped" as const, archived_at: null },
      {
        ...primarySessionFixture,
        id: "archived",
        state: "stopped" as const,
        archived_at: "2026-09-11",
      },
    ];
    expect(sessionSelectionCounts(sessions, ["active", "stopped"])).toEqual({
      stoppable: 2,
      archivable: 1,
      unarchivable: 1,
      hiddenByFilter: 2,
    });
  });
});
