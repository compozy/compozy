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
import { windowManagerSettingsConfigToWire } from "../../lib/window-manager-layout-schema";
import {
  applyTerminalShortcutPreset,
  previewTerminalShortcutPreset,
  revertTerminalShortcutPreset,
  TERMINAL_SHORTCUT_PRESET,
} from "../../lib/window-manager-shortcut-preset";

import {
  useWindowManagerConfigEditor,
  windowManagerConfigEditorLogic,
} from "../use-window-manager-config-editor";

const SHORTCUT_DEFAULTS = {
  "window.close": ["meta+KeyW"],
  "window.tile.left": ["control+alt+ArrowLeft"],
  "window.focus.up": ["control+ArrowUp"],
  "window.focus.down": ["control+ArrowDown"],
  "window.tile.bottom-left": ["control+alt+KeyJ"],
  "window.tile.bottom-right": ["control+alt+KeyK"],
  "sidebar.toggle": ["meta+KeyB"],
  "window.tab.jump.1": ["meta+Digit1"],
  "window.tab.jump.2": ["meta+Digit2"],
  "window.tab.jump.3": ["meta+Digit3"],
  "window.tab.jump.4": ["meta+Digit4"],
  "window.tab.jump.5": ["meta+Digit5"],
  "window.tab.jump.6": ["meta+Digit6"],
  "window.tab.jump.7": ["meta+Digit7"],
  "window.tab.jump.8": ["meta+Digit8"],
  "window.tab.last": ["meta+Digit9"],
  "desktop.switch.1": ["control+Digit1"],
  "desktop.switch.2": ["control+Digit2"],
  "desktop.switch.3": ["control+Digit3"],
  "desktop.switch.4": ["control+Digit4"],
  "desktop.switch.5": ["control+Digit5"],
  "desktop.switch.6": ["control+Digit6"],
  "desktop.switch.7": ["control+Digit7"],
  "desktop.switch.8": ["control+Digit8"],
  "desktop.switch.9": ["control+Digit9"],
} as const;

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
  navStackLimit: 50,
  closedEntryLimit: 20,
  desktopTransition: "slide",
  gaps: { inner: 8, top: 8, right: 8, bottom: 8, left: 8 },
  snap: { edgeBand: 24, cornerReach: 96, exitSlack: 16, repeatRatios: [0.5, 0.33, 0.67] },
  bindings: { topCenter: "zoom", bottomCenter: "none" },
  shortcuts: {},
  shortcutDefaults: SHORTCUT_DEFAULTS,
  effectiveShortcuts: SHORTCUT_DEFAULTS,
};

function renderEditor(initial = CONFIG) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  return renderHook(({ baseline }) => useWindowManagerConfigEditor(baseline), {
    initialProps: { baseline: initial },
    wrapper,
  });
}

function fields(problems: ReadonlyArray<{ field: string }>) {
  return problems.map(problem => problem.field);
}

describe("useWindowManagerConfigEditor", () => {
  it("Should preserve every daemon-required limit in the wire config", () => {
    expect(windowManagerSettingsConfigToWire(CONFIG)).toMatchObject({
      history_limit: 100,
      nav_stack_limit: 50,
      closed_entry_limit: 20,
    });
  });

  it("Should preserve a dirty draft when a newer baseline is rendered", () => {
    const { result, rerender } = renderEditor();
    act(() => {
      result.current.setDraft(current => ({ ...current, historyLimit: 101 }));
    });
    rerender({ baseline: { ...CONFIG, historyLimit: 99 } });

    expect(result.current.draft.historyLimit).toBe(101);
    expect(result.current.phase).toBe("dirty");
  });

  it("Should ignore a late save result after a newer draft transition", () => {
    const store = windowManagerConfigEditorLogic.createStore({ baseline: CONFIG });
    let snapshot = store.getInitialSnapshot();
    const revision = JSON.stringify(CONFIG);
    [snapshot] = store.transition(snapshot, {
      type: "draftChanged",
      draft: { ...CONFIG, historyLimit: 101 },
    });
    [snapshot] = store.transition(snapshot, {
      type: "saveRequested",
      execute: async () => undefined,
    });
    [snapshot] = store.transition(snapshot, {
      type: "draftChanged",
      draft: { ...CONFIG, historyLimit: 102 },
    });
    [snapshot] = store.transition(snapshot, {
      type: "saveSucceeded",
      operation: 1,
      revision,
      draftRevision: 1,
    });

    expect(snapshot.context.draft.historyLimit).toBe(102);
    expect(snapshot.context.phase).toBe("dirty");
  });

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
        "window.focus.up": ["meta+shift+KeyP"],
        "window.focus.down": ["meta+shift+KeyP"],
      });
    });

    expect(fields(result.current.problems)).toEqual(["shortcuts"]);
    expect(result.current.canSave).toBe(false);
  });

  it("Should block an override that collides with a daemon default", () => {
    const { result } = renderEditor();
    act(() => {
      result.current.setShortcuts({ "window.close": ["control+alt+ArrowLeft"] });
    });

    expect(fields(result.current.problems)).toEqual(["shortcuts"]);
    expect(result.current.canSave).toBe(false);
  });

  it("Should keep focused-surface shadowing advisory rather than blocking", () => {
    const { result } = renderEditor();
    act(() => result.current.setShortcuts({ "sidebar.toggle": ["meta+KeyB"] }));

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

/**
 * The preset preview reports rows by label, so it needs the catalog the daemon
 * would project — every id the preset touches, plus the defaults it displaces.
 */
const PRESET_ACTIONS = [
  ...new Set([...Object.keys(TERMINAL_SHORTCUT_PRESET), ...Object.keys(SHORTCUT_DEFAULTS)]),
].map(id => ({ id, label: id, section: "Window" }));

describe("Terminal shortcut preset", () => {
  it("Should preview every changed key and flag layout hazards [UT-063]", () => {
    const preview = previewTerminalShortcutPreset({}, SHORTCUT_DEFAULTS, PRESET_ACTIONS);

    expect(preview.map(change => change.actionId)).toEqual(Object.keys(TERMINAL_SHORTCUT_PRESET));
    expect(preview.some(change => change.hazards.includes("altgr"))).toBe(true);
    expect(
      preview.find(change => change.actionId === "window.tile.bottom-left")?.hazards
    ).toContain("displaced-default");
    expect(preview.find(change => change.actionId === "window.tab.jump")?.before).toEqual([
      "meta+Digit1..8",
    ]);
    expect(preview.find(change => change.actionId === "desktop.switch")?.after).toEqual([
      "control+Digit1..9",
      "meta+Digit1..9",
    ]);
    const conflicted = previewTerminalShortcutPreset(
      { "layout.undo": ["control+ArrowLeft"] },
      SHORTCUT_DEFAULTS,
      PRESET_ACTIONS
    );
    expect(
      conflicted
        .find(change => change.actionId === "window.focus.left")
        ?.conflicts.some(conflict => conflict.kind === "blocked")
    ).toBe(true);
  });

  it("Should apply atomically, re-apply idempotently, and restore only touched keys [UT-064]", () => {
    const before = {
      "window.close": ["meta+shift+KeyW"],
      "layout.undo": ["alt+KeyU"],
    };
    const applied = applyTerminalShortcutPreset(before);
    expect(applied.changed).toBe(true);
    expect(applied.revert).not.toBeNull();
    expect(applied.overrides["window.close"]).toEqual(TERMINAL_SHORTCUT_PRESET["window.close"]);

    const reapplied = applyTerminalShortcutPreset(applied.overrides);
    expect(reapplied.changed).toBe(false);
    expect(reapplied.overrides).toBe(applied.overrides);

    const manuallyEdited = { ...applied.overrides, "layout.undo": ["alt+KeyZ"] };
    const restored = revertTerminalShortcutPreset(manuallyEdited, applied.revert!);
    expect(restored["window.close"]).toEqual(["meta+shift+KeyW"]);
    expect(restored["layout.undo"]).toEqual(["alt+KeyZ"]);
  });
});
