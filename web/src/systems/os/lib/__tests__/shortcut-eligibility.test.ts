import { afterEach, describe, expect, it } from "vitest";

import {
  isShortcutEligible,
  shortcutSurface,
  type ShortcutKeyEvent,
  type ShortcutSurface,
} from "../shortcut-eligibility";

// Invariant: one policy decides, for the document listener and the terminal alike,
// whether a keystroke matching a bound chord may run its command from the focused surface.

interface Press {
  key?: string;
  code: string;
  meta?: boolean;
  ctrl?: boolean;
  alt?: boolean;
  shift?: boolean;
  composing?: boolean;
  altGraph?: boolean;
}

function press({ key, code, meta, ctrl, alt, shift, composing, altGraph }: Press): ShortcutKeyEvent {
  return {
    key: key ?? "Unidentified",
    code,
    metaKey: meta ?? false,
    ctrlKey: ctrl ?? false,
    altKey: alt ?? false,
    shiftKey: shift ?? false,
    isComposing: composing ?? false,
    getModifierState: key => key === "AltGraph" && (altGraph ?? false),
  };
}

function eligible(surface: ShortcutSurface, apple: boolean, event: Press): boolean {
  return isShortcutEligible(press(event), surface, apple);
}

describe("shortcut eligibility", () => {
  afterEach(() => {
    document.body.replaceChildren();
  });

  it("Should classify the focused surface, with the terminal winning over its helper textarea", () => {
    const terminal = document.createElement("div");
    terminal.dataset.slot = "terminal-view";
    const helper = document.createElement("textarea");
    terminal.append(helper);
    const editable = document.createElement("div");
    editable.setAttribute("contenteditable", "true");
    const nested = document.createElement("span");
    editable.append(nested);
    const readOnly = document.createElement("div");
    readOnly.setAttribute("contenteditable", "false");
    const button = document.createElement("button");
    document.body.append(terminal, editable, readOnly, button, document.createElement("input"));

    expect(shortcutSurface(helper)).toBe("terminal");
    expect(shortcutSurface(nested)).toBe("text");
    expect(shortcutSurface(document.querySelector("input"))).toBe("text");
    expect(shortcutSurface(readOnly)).toBe("none");
    expect(shortcutSurface(button)).toBe("none");
    expect(shortcutSurface(null)).toBe("none");
  });

  it("Should keep unmodified, shift-only and composing keys as typing in fields and the terminal", () => {
    for (const surface of ["text", "terminal"] as const) {
      expect(eligible(surface, true, { code: "Slash", shift: true })).toBe(false);
      expect(eligible(surface, true, { code: "KeyA", shift: true })).toBe(false);
      expect(eligible(surface, true, { code: "KeyA" })).toBe(false);
      expect(eligible(surface, true, { code: "KeyH", alt: true, composing: true })).toBe(false);
    }
    expect(eligible("none", true, { code: "Slash", shift: true })).toBe(true);
    expect(eligible("none", true, { code: "KeyH", alt: true, composing: true })).toBe(false);
  });

  it("Should treat AltGr as typing off Apple and Option as a modifier on Apple", () => {
    const altGr = { code: "KeyQ", ctrl: true, alt: true, altGraph: true };
    expect(eligible("text", false, altGr)).toBe(false);
    expect(eligible("none", false, altGr)).toBe(false);
    expect(eligible("text", false, { code: "KeyQ", ctrl: true, alt: true })).toBe(true);
    expect(eligible("text", true, altGr)).toBe(true);
  });

  it("Should leave characters a Mac layout types with Option to a field and the terminal", () => {
    // German ⌥L and Italian ⌥ò (the Semicolon key) type "@"; German ⌥5 types "[".
    const typed = [
      { key: "@", code: "KeyL", alt: true },
      { key: "@", code: "Semicolon", alt: true },
      { key: "[", code: "Digit5", alt: true },
      { key: "|", code: "Digit7", alt: true, shift: true },
    ];
    for (const event of typed) {
      expect(eligible("text", true, event)).toBe(false);
      expect(eligible("terminal", true, event)).toBe(false);
      expect(eligible("none", true, event)).toBe(true);
    }
    // US glyphs are not ASCII, and an Option that sends the key's own character acts as Alt.
    expect(eligible("text", true, { key: "˙", code: "KeyH", alt: true })).toBe(true);
    expect(eligible("text", true, { key: "…", code: "Semicolon", alt: true })).toBe(true);
    expect(eligible("text", true, { key: "Ú", code: "Semicolon", alt: true, shift: true })).toBe(true);
    expect(eligible("terminal", true, { key: "l", code: "KeyL", alt: true })).toBe(true);
    expect(eligible("terminal", true, { key: ":", code: "Semicolon", alt: true, shift: true })).toBe(
      true
    );
    // With Control or Command held the chord is never a typed character.
    expect(eligible("text", true, { key: "@", code: "KeyL", alt: true, ctrl: true })).toBe(true);
  });

  it("Should leave native editing chords and caret moves to a text field by platform", () => {
    // Apple: ⌘ is primary, ⌥/⌘ + arrows move the caret, ⌃ + arrows are free.
    expect(eligible("text", true, { code: "KeyZ", meta: true })).toBe(false);
    expect(eligible("text", true, { code: "KeyZ", meta: true, shift: true })).toBe(false);
    for (const code of ["KeyA", "KeyC", "KeyV", "KeyX", "KeyB", "KeyI", "KeyU"]) {
      expect(eligible("text", true, { code, meta: true })).toBe(false);
    }
    expect(eligible("text", true, { code: "ArrowLeft", alt: true })).toBe(false);
    expect(eligible("text", true, { code: "ArrowLeft", meta: true, shift: true })).toBe(false);
    expect(eligible("text", true, { code: "Backspace", alt: true })).toBe(false);
    expect(eligible("text", true, { code: "ArrowLeft", ctrl: true })).toBe(true);
    expect(eligible("text", true, { code: "ArrowLeft", ctrl: true, alt: true })).toBe(true);
    expect(eligible("text", true, { code: "KeyW", meta: true })).toBe(true);
    expect(eligible("text", true, { code: "KeyT", meta: true, shift: true })).toBe(true);
    expect(eligible("text", true, { code: "KeyH", alt: true })).toBe(true);

    // Elsewhere: Ctrl is primary and Ctrl + arrows move by word.
    expect(eligible("text", false, { code: "KeyZ", ctrl: true })).toBe(false);
    expect(eligible("text", false, { code: "KeyB", ctrl: true })).toBe(false);
    expect(eligible("text", false, { code: "ArrowLeft", ctrl: true })).toBe(false);
    expect(eligible("text", false, { code: "ArrowLeft", ctrl: true, shift: true })).toBe(false);
    expect(eligible("text", false, { code: "ArrowLeft", alt: true })).toBe(true);
    expect(eligible("text", false, { code: "KeyZ", meta: true })).toBe(true);
  });

  it("Should give the shell its Ctrl chords off Apple and every unbound chord on Apple", () => {
    // Apple: any bound chord with ⌘, ⌃ or ⌥ runs; the caller only asks about bound chords.
    expect(eligible("terminal", true, { code: "KeyW", ctrl: true })).toBe(true);
    expect(eligible("terminal", true, { code: "KeyH", alt: true })).toBe(true);
    expect(eligible("terminal", true, { code: "KeyW", meta: true })).toBe(true);

    expect(eligible("terminal", false, { code: "KeyW", ctrl: true })).toBe(false);
    expect(eligible("terminal", false, { code: "Digit1", ctrl: true })).toBe(false);
    expect(eligible("terminal", false, { code: "KeyK", ctrl: true })).toBe(true);
    expect(eligible("terminal", false, { code: "Tab", ctrl: true, shift: true })).toBe(true);
    expect(eligible("terminal", false, { code: "ArrowDown", ctrl: true, alt: true })).toBe(true);
    expect(eligible("terminal", false, { code: "KeyH", alt: true })).toBe(true);
  });
});
