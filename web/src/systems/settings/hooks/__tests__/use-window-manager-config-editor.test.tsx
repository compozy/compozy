// Suite: window-manager config validity
// Invariant: a draft the daemon would refuse cannot be saved, and the editor can
// say which value is at fault rather than only that one is. Ranges mirror the
// web contract; the shortcut rule mirrors `CanonicalShortcuts`, which rejects a
// chord claimed by two overrides but accepts one that shadows a shipped default.
// Boundary IN: a config draft.
// Boundary OUT: the controls that produce it, transport.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it } from "vitest";

import type { WindowManagerConfig } from "@/systems/os";

import { useWindowManagerConfigEditor } from "../use-window-manager-config-editor";

const CONFIG: WindowManagerConfig = {
  newWindowPolicy: "floating",
  smallViewportPolicy: "stack",
  focusPolicy: "click_directional",
  focusWrap: true,
  focusFollowsPointer: false,
  raiseOnFocus: true,
  dragAwayPolicy: "window",
  groupMoveModifier: "alt",
  swapModifier: "shift",
  historyLimit: 100,
  desktopTransition: "slide",
  gaps: { inner: 8, top: 8, right: 8, bottom: 8, left: 8 },
  snap: { edgeBand: 24, cornerReach: 96, exitSlack: 16, repeatRatios: [0.5, 0.33, 0.67] },
  bindings: { topCenter: "zoom", bottomCenter: "none" },
  shortcuts: {},
};

function renderEditor() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  return renderHook(() => useWindowManagerConfigEditor(CONFIG), { wrapper });
}

function fields(problems: ReadonlyArray<{ field: string }>) {
  return problems.map(problem => problem.field);
}

describe("useWindowManagerConfigEditor", () => {
  it("Should allow saving a changed draft that is within range", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setDraft(current => ({ ...current, historyLimit: 101 }));
    });

    expect(result.current.problems).toEqual([]);
    expect(result.current.canSave).toBe(true);
  });

  it("Should name history limit when it leaves its range", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setDraft(current => ({ ...current, historyLimit: 501 }));
    });

    expect(fields(result.current.problems)).toEqual(["historyLimit"]);
    expect(result.current.canSave).toBe(false);
  });

  it("Should reject duplicate repeat widths at the daemon's precision", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setDraft(current => ({
        ...current,
        snap: { ...current.snap, repeatRatios: [0.5, 0.5000001] },
      }));
    });

    expect(fields(result.current.problems)).toEqual(["repeatRatios"]);
    expect(result.current.canSave).toBe(false);
  });

  it("Should name gaps and snap separately when both are out of range", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setDraft(current => ({
        ...current,
        gaps: { ...current.gaps, inner: 90 },
        snap: { ...current.snap, edgeBand: 2 },
      }));
    });

    expect(fields(result.current.problems)).toEqual(["gaps", "snap"]);
  });

  it("Should block a save when two overrides claim the same chord", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setShortcuts({
        "window.focus.up": "meta+shift+KeyP",
        "window.focus.down": "meta+shift+KeyP",
      });
    });

    expect(fields(result.current.problems)).toEqual(["shortcuts"]);
    expect(result.current.canSave).toBe(false);
  });

  it("Should allow an override that only shadows another action's shipped chord", () => {
    const { result } = renderEditor();
    act(() => {
      // control+alt+ArrowLeft ships on window.tile.left; the daemon stores this.
      result.current.setShortcuts({ "window.close": "control+alt+ArrowLeft" });
    });

    expect(result.current.problems).toEqual([]);
    expect(result.current.canSave).toBe(true);
  });

  it("Should return to the saved config on reset", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setDraft(current => ({ ...current, historyLimit: 7 }));
    });
    expect(result.current.dirty).toBe(true);

    act(() => {
      result.current.reset();
    });

    expect(result.current.dirty).toBe(false);
    expect(result.current.draft.historyLimit).toBe(100);
  });
});
