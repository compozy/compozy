// Suite: window-manager chord grammar
// Invariant: chords read from the keyboard, parsed from storage, and rendered to
// the user all agree with the daemon's canonical form — modifier order
// meta+control+alt+shift, exactly one allowlisted KeyboardEvent.code, at least one
// modifier (internal/windowmanager/shortcuts.go: canonicalShortcutChord) — and the
// two conflict classes the daemon and the shell treat differently stay separable.
// Boundary IN: an overrides map or one keypress.
// Boundary OUT: recording lifecycle, React state, transport.
import { describe, expect, it } from "vitest";

import { WINDOW_MANAGER_ACTIONS } from "../window-manager-command-registry";
import {
  chordFromKeyboardEvent,
  findShortcutConflicts,
  parseShortcutChord,
  resolveWindowManagerActions,
  shortcutLabel,
} from "../window-manager-shortcuts";

function press(init: KeyboardEventInit & { code: string }): KeyboardEvent {
  return new KeyboardEvent("keydown", { key: init.code, ...init });
}

describe("chordFromKeyboardEvent", () => {
  it("Should emit modifiers in the daemon's canonical order", () => {
    const chord = press({
      code: "KeyB",
      altKey: true,
      ctrlKey: true,
      metaKey: true,
      shiftKey: true,
    });
    expect(chordFromKeyboardEvent(chord)).toBe("meta+control+alt+shift+KeyB");
  });

  it("Should keep listening while only modifiers are held", () => {
    expect(chordFromKeyboardEvent(press({ code: "ShiftLeft", key: "Shift", shiftKey: true }))).toBe(
      null
    );
    expect(chordFromKeyboardEvent(press({ code: "MetaLeft", key: "Meta", metaKey: true }))).toBe(
      null
    );
  });

  it("Should reject a chord with no modifier", () => {
    expect(chordFromKeyboardEvent(press({ code: "KeyB" }))).toBe(null);
  });

  it("Should reject a code the daemon does not accept", () => {
    expect(chordFromKeyboardEvent(press({ code: "F13", ctrlKey: true }))).toBe(null);
    expect(chordFromKeyboardEvent(press({ code: "NumpadEnter", ctrlKey: true }))).toBe(null);
  });

  it("Should round-trip through the parser it shares with stored overrides", () => {
    const recorded = chordFromKeyboardEvent(
      press({ code: "ArrowLeft", ctrlKey: true, altKey: true })
    );
    expect(recorded).toBe("control+alt+ArrowLeft");
    expect(parseShortcutChord(recorded ?? "")?.canonical).toBe(recorded);
    expect(shortcutLabel(parseShortcutChord(recorded ?? "")!)).toBe("⌃⌥←");
  });
});

describe("findShortcutConflicts", () => {
  it("Should report no conflict for the shipped defaults", () => {
    expect(findShortcutConflicts({})).toEqual([]);
  });

  it("Should block when two overrides claim the same chord", () => {
    const conflicts = findShortcutConflicts({
      "window.focus.up": "meta+shift+KeyP",
      "window.focus.down": "meta+shift+KeyP",
    });
    expect(conflicts).toHaveLength(1);
    expect(conflicts[0]?.kind).toBe("override");
    expect(conflicts[0]?.chord).toBe("meta+shift+KeyP");
    expect([...(conflicts[0]?.actionIds ?? [])].sort()).toEqual([
      "window.focus.down",
      "window.focus.up",
    ]);
  });

  it("Should mark an override landing on another action's shipped chord as shadowing", () => {
    // control+alt+ArrowLeft ships on window.tile.left.
    const conflicts = findShortcutConflicts({ "window.close": "control+alt+ArrowLeft" });
    expect(conflicts).toHaveLength(1);
    expect(conflicts[0]?.kind).toBe("default");
    expect(conflicts[0]?.actionIds).toContain("window.tile.left");
    expect(conflicts[0]?.actionIds).toContain("window.close");
  });

  it("Should ignore an override that only restates its own default", () => {
    expect(findShortcutConflicts({ "window.close": "meta+KeyW" })).toEqual([]);
  });

  it("Should ignore actions left unbound", () => {
    const unbound = WINDOW_MANAGER_ACTIONS.filter(action => !action.defaultChord);
    expect(unbound.length).toBeGreaterThan(0);
    expect(resolveWindowManagerActions({}).filter(action => !action.chord)).toHaveLength(
      unbound.length
    );
    expect(findShortcutConflicts({})).toEqual([]);
  });
});
