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
  primaryShortcutModifier,
  resolveWindowManagerActions,
  shortcutMatches,
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

  it("Should surface a tab-action override that collides with close [UT-051]", () => {
    const conflicts = findShortcutConflicts({ "window.tab.next": "meta+KeyW" });

    expect(conflicts).toEqual([
      {
        chord: "meta+KeyW",
        kind: "default",
        actionIds: ["window.close", "window.tab.next"],
      },
    ]);
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

describe("shortcutMatches", () => {
  it("Should map the canonical primary modifier to Command on Apple and Control elsewhere", () => {
    const primary = parseShortcutChord("meta+KeyT");
    const physicalControl = parseShortcutChord("control+KeyT");
    if (!primary || !physicalControl) throw new Error("test chords must parse");

    const commandT = press({ code: "KeyT", metaKey: true });
    const controlT = press({ code: "KeyT", ctrlKey: true });

    expect(primaryShortcutModifier("MacIntel")).toBe("meta");
    expect(primaryShortcutModifier("Linux x86_64")).toBe("control");
    expect(primaryShortcutModifier("")).toBe("meta");
    expect(shortcutMatches(commandT, primary, "meta")).toBe(true);
    expect(shortcutMatches(controlT, primary, "control")).toBe(true);
    expect(shortcutMatches(commandT, primary, "control")).toBe(false);
    expect(shortcutMatches(commandT, physicalControl, "meta")).toBe(false);
    expect(shortcutMatches(controlT, physicalControl, "meta")).toBe(true);
  });

  it("Should preserve distinct Meta and Control requirements when a chord contains both", () => {
    const chord = parseShortcutChord("meta+control+KeyT");
    if (!chord) throw new Error("test chord must parse");

    expect(shortcutMatches(press({ code: "KeyT", ctrlKey: true }), chord, "control")).toBe(false);
    expect(
      shortcutMatches(press({ code: "KeyT", ctrlKey: true, metaKey: true }), chord, "control")
    ).toBe(true);
  });
});
