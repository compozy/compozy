// Suite: window-manager shortcut grammar
// Invariant: the chord layer normalizes arrays/ranges exactly once, binds only
// the actions its caller supplies against daemon-fed chords, and reports every
// expanded collision before Settings can save it.
// Owning layer: OS chord grammar. The command inventory itself is the daemon's
// and reaches this layer as a parameter (cmd-palette registry suites own it).
// Boundary OUT: recorder state, React rendering, and transport parsing.
import { describe, expect, it } from "vitest";

import {
  chordFromKeyboardEvent,
  coveringShortcutFamily,
  deriveShortcutCheatsheet,
  effectiveShortcutMap,
  findShortcutConflicts,
  parseShortcutChord,
  primaryShortcutModifier,
  resolveWindowManagerActions,
  shortcutBindingProblem,
  shortcutKeyGlyphs,
  shortcutMatches,
  shortcutLabel,
  SHORTCUT_RANGE_FAMILIES,
  type ShortcutActionDefinition,
  type ShortcutMap,
} from "../window-manager-shortcuts";

function coreAction(
  action: Omit<ShortcutActionDefinition, "source" | "alias">
): ShortcutActionDefinition {
  return { source: "core", alias: null, ...action };
}

/** The catalog the daemon would project; this layer only ever receives it. */
const ACTIONS: readonly ShortcutActionDefinition[] = [
  coreAction({ id: "window.close", label: "Close window", section: "Window" }),
  coreAction({ id: "window.tile.left", label: "Tile left half", section: "Tiling" }),
  coreAction({ id: "window.focus.up", label: "Focus up", section: "Window" }),
  coreAction({ id: "window.focus.down", label: "Focus down", section: "Window" }),
  coreAction({ id: "sidebar.toggle", label: "Toggle sidebar", section: "Shell" }),
  ...Array.from({ length: 9 }, (_, index) =>
    coreAction({
      id: `desktop.switch.${index + 1}`,
      label: `Switch to desktop ${index + 1}`,
      section: "Desktops",
    })
  ),
  ...Array.from({ length: 8 }, (_, index) =>
    coreAction({
      id: `window.tab.jump.${index + 1}`,
      label: `Go to tab ${index + 1}`,
      section: "Tabs",
    })
  ),
  coreAction({ id: "layout.arrange.grid", label: "Arrange in grid", section: "Layout" }),
  {
    id: "ext.notes.capture",
    label: "Capture note",
    section: "Notes",
    source: "ext.notes",
    alias: "cap",
  },
];

const DEFAULTS: ShortcutMap = {
  "window.close": ["meta+KeyW"],
  "window.tile.left": ["control+alt+ArrowLeft"],
  "window.focus.up": ["control+ArrowUp"],
  "window.focus.down": ["control+ArrowDown"],
  "sidebar.toggle": ["meta+KeyB"],
  "desktop.switch.1": ["control+Digit1"],
  "desktop.switch.2": ["control+Digit2"],
  "window.tab.jump.1": ["meta+Digit1"],
  "window.tab.jump.2": ["meta+Digit2"],
  "layout.arrange.grid": [],
};

function press(init: KeyboardEventInit & { code: string }): KeyboardEvent {
  return new KeyboardEvent("keydown", { key: init.code, ...init });
}

describe("chord grammar [UT-062]", () => {
  it("Should canonicalize keyboard input and share one glyph helper", () => {
    const chord = press({
      code: "KeyB",
      altKey: true,
      ctrlKey: true,
      metaKey: true,
      shiftKey: true,
    });
    expect(chordFromKeyboardEvent(chord)).toBe("meta+control+alt+shift+KeyB");
    expect(shortcutKeyGlyphs("meta+control+alt+shift+KeyB")).toEqual(["⌘", "⌃", "⌥", "⇧", "B"]);
    expect(shortcutLabel("control+alt+ArrowLeft")).toBe("⌃⌥←");
  });

  it("Should reject bare, modifier-only, and unsupported keys", () => {
    expect(chordFromKeyboardEvent(press({ code: "KeyB" }))).toBe(null);
    expect(chordFromKeyboardEvent(press({ code: "ShiftLeft", key: "Shift", shiftKey: true }))).toBe(
      null
    );
    expect(chordFromKeyboardEvent(press({ code: "F13", ctrlKey: true }))).toBe(null);
  });

  it("Should validate arrays, empty disables, and indexed ranges", () => {
    expect(shortcutBindingProblem("window.focus.up", ["control+ArrowUp", "alt+KeyK"])).toBeNull();
    expect(shortcutBindingProblem("window.focus.up", [])).toBeNull();
    expect(shortcutBindingProblem("window.focus.up", [""])).toBeNull();
    expect(
      shortcutBindingProblem("desktop.switch", ["control+Digit1..9", "meta+Digit1..9"])
    ).toBeNull();
    expect(shortcutBindingProblem("window.tab.jump", ["control+Digit1..9"])).toContain(
      "1 through 8"
    );
    expect(shortcutBindingProblem("window.close", ["control+Digit1..8"])).toContain("indexed");
  });

  it("Should render portable primary chords and compact range glyphs", () => {
    expect(shortcutLabel("meta+shift+KeyP", "control")).toBe("⌃⇧P");
    expect(shortcutLabel("control+Digit1..9")).toBe("⌃1–9");
  });

  it("Should expand range alternates and replace the whole touched family", () => {
    const effective = effectiveShortcutMap(DEFAULTS, {
      "desktop.switch": ["alt+Digit1..2", "meta+control+Digit1..2"],
    });
    expect(effective["desktop.switch.1"]).toEqual(["alt+Digit1", "meta+control+Digit1"]);
    expect(effective["desktop.switch.2"]).toEqual(["alt+Digit2", "meta+control+Digit2"]);
    expect(effective["desktop.switch.3"]).toBeUndefined();
  });
});

describe("effective conflicts", () => {
  it("Should report no collision for the daemon defaults", () => {
    expect(
      findShortcutConflicts({}, DEFAULTS).filter(conflict => conflict.kind === "blocked")
    ).toEqual([]);
  });

  it("Should block duplicates across overrides and defaults after expansion", () => {
    const duplicateOverrides = findShortcutConflicts(
      {
        "window.focus.up": ["meta+shift+KeyP"],
        "window.focus.down": ["meta+shift+KeyP"],
      },
      DEFAULTS
    );
    expect(duplicateOverrides.find(conflict => conflict.kind === "blocked")).toMatchObject({
      chord: "meta+shift+KeyP",
      kind: "blocked",
    });

    const defaultCollision = findShortcutConflicts(
      { "window.close": ["control+alt+ArrowLeft"] },
      DEFAULTS
    );
    expect(defaultCollision).toContainEqual({
      chord: "control+alt+ArrowLeft",
      kind: "blocked",
      actionIds: ["window.close", "window.tile.left"],
    });
  });

  it("Should name focused-surface shadowing without blocking the map", () => {
    expect(findShortcutConflicts({}, DEFAULTS)).toContainEqual({
      chord: "meta+KeyB",
      kind: "shadowed",
      actionIds: ["sidebar.toggle"],
      winner: { label: "Bold", surface: "composer" },
    });
  });
});

describe("daemon-fed registry", () => {
  it("Should bind no chord when the daemon keymap is empty [IT-015 parity]", () => {
    // The chord layer holds no defaults of its own: with an empty effective
    // keymap every action resolves unbound, whatever the catalog says.
    expect(
      resolveWindowManagerActions({}, ACTIONS).every(action => action.chords.length === 0)
    ).toBe(true);
  });

  it("Should name the indexed family that covers a member when that family is overridden", () => {
    const overrides: ShortcutMap = { "window.tab.jump": ["control+alt+Digit1..8"] };
    expect(coveringShortcutFamily("window.tab.jump.3", overrides)).toBe("window.tab.jump");
    expect(coveringShortcutFamily("desktop.switch.2", overrides)).toBeNull();
    expect(SHORTCUT_RANGE_FAMILIES.map(family => family.action)).toEqual([
      "desktop.switch",
      "window.tab.jump",
    ]);
  });

  it("Should surface arrays once and collapse numeric families in the cheatsheet [UT-075]", () => {
    const effective = effectiveShortcutMap(DEFAULTS, {
      "window.focus.up": ["control+ArrowUp", "alt+KeyK"],
      "window.tab.jump": ["control+alt+Digit1..2"],
    });
    const rows = deriveShortcutCheatsheet(
      effective,
      {
        "window.focus.up": ["control+ArrowUp", "alt+KeyK"],
        "window.tab.jump": ["control+alt+Digit1..2"],
      },
      ACTIONS
    );
    expect(rows.filter(row => row.id === "window.focus.up")).toEqual([
      expect.objectContaining({ bindings: ["control+ArrowUp", "alt+KeyK"], overridden: true }),
    ]);
    expect(rows.filter(row => row.id === "window.tab.jump")).toHaveLength(1);
    expect(rows.some(row => row.id.startsWith("window.tab.jump."))).toBe(false);
  });

  it("Should preserve caller-supplied extension source and alias", () => {
    const effective = effectiveShortcutMap(DEFAULTS, {
      "ext.notes.capture": ["alt+shift+KeyN"],
    });
    const resolved = resolveWindowManagerActions(effective, ACTIONS);
    expect(resolved.find(action => action.id === "ext.notes.capture")).toEqual(
      expect.objectContaining({
        id: "ext.notes.capture",
        source: "ext.notes",
        alias: "cap",
        shortcutLabels: ["⌥⇧N"],
      })
    );
    expect(
      deriveShortcutCheatsheet(
        effective,
        { "ext.notes.capture": ["alt+shift+KeyN"] },
        ACTIONS
      ).find(row => row.id === "ext.notes.capture")
    ).toEqual(
      expect.objectContaining({
        source: "ext.notes",
        alias: "cap",
        bindings: ["alt+shift+KeyN"],
        overridden: true,
      })
    );
  });

  it("Should report a collision that includes an unhydrated action id", () => {
    expect(findShortcutConflicts({ "ext.unknown.open": ["meta+KeyW"] }, DEFAULTS)).toContainEqual({
      chord: "meta+KeyW",
      kind: "blocked",
      actionIds: ["window.close", "ext.unknown.open"],
    });
  });
});

describe("shortcutMatches", () => {
  it("Should map portable primary chords while preserving explicit control", () => {
    const primary = parseShortcutChord("meta+KeyT");
    const physicalControl = parseShortcutChord("control+KeyT");
    if (!primary || !physicalControl) throw new Error("test chords must parse");
    const commandT = press({ code: "KeyT", metaKey: true });
    const controlT = press({ code: "KeyT", ctrlKey: true });
    expect(primaryShortcutModifier("MacIntel")).toBe("meta");
    expect(primaryShortcutModifier("Linux x86_64")).toBe("control");
    expect(shortcutMatches(commandT, primary, "meta")).toBe(true);
    expect(shortcutMatches(controlT, primary, "control")).toBe(true);
    expect(shortcutMatches(commandT, physicalControl, "meta")).toBe(false);
    expect(shortcutMatches(controlT, physicalControl, "meta")).toBe(true);
  });
});
