/**
 * Which surface owns the focused keystroke.
 *
 * - `text` — an input, textarea, select or contenteditable (the Lexical composer).
 * - `terminal` — anything inside a terminal view, including xterm's helper textarea.
 * - `none` — the shell itself.
 */
export type ShortcutSurface = "text" | "terminal" | "none";

/** The slice of a keyboard event the policy reads; a DOM `KeyboardEvent` satisfies it. */
export interface ShortcutKeyEvent {
  readonly code: string;
  readonly metaKey: boolean;
  readonly ctrlKey: boolean;
  readonly altKey: boolean;
  readonly shiftKey: boolean;
  readonly isComposing: boolean;
  getModifierState(key: string): boolean;
}

const CARET_CODES: ReadonlySet<string> = new Set([
  "ArrowLeft",
  "ArrowRight",
  "ArrowUp",
  "ArrowDown",
  "Home",
  "End",
  "Backspace",
  "Delete",
]);

/** Editing chords a text control owns: select all, clipboard, bold/italic/underline. */
const TEXT_EDITING_CODES: ReadonlySet<string> = new Set([
  "KeyA",
  "KeyC",
  "KeyV",
  "KeyX",
  "KeyB",
  "KeyI",
  "KeyU",
]);

export function shortcutSurface(target: EventTarget | null): ShortcutSurface {
  if (!(target instanceof Element)) return "none";
  if (target.closest('[data-slot="terminal-view"]')) return "terminal";
  const editable = target.closest("input, textarea, select, [contenteditable]");
  if (!(editable instanceof HTMLElement)) return "none";
  if (
    editable.isContentEditable ||
    (editable.hasAttribute("contenteditable") &&
      editable.getAttribute("contenteditable") !== "false") ||
    editable instanceof HTMLInputElement ||
    editable instanceof HTMLTextAreaElement ||
    editable instanceof HTMLSelectElement
  ) {
    return "text";
  }
  return "none";
}

/** Undo/redo, select all, clipboard, formatting and caret moves stay with the text control. */
function isNativeTextChord(event: ShortcutKeyEvent, apple: boolean): boolean {
  const primary = apple ? event.metaKey : event.ctrlKey;
  const secondary = apple ? event.ctrlKey : event.metaKey;
  if (primary && !secondary && !event.altKey) {
    if (event.code === "KeyZ") return true;
    if (!event.shiftKey && TEXT_EDITING_CODES.has(event.code)) return true;
  }
  if (!CARET_CODES.has(event.code)) return false;
  if (apple) return !event.ctrlKey && event.altKey !== event.metaKey;
  return event.ctrlKey && !event.altKey && !event.metaKey;
}

/**
 * Off Apple a chord whose only modifier is Ctrl belongs to the shell (⌃W, ⌃E, ⌃Z, ⌃1…),
 * except the palette chord ⌃K the terminal already released before this policy existed.
 */
function isShellControlChord(event: ShortcutKeyEvent): boolean {
  return (
    event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey && event.code !== "KeyK"
  );
}

/**
 * Whether a keystroke that matches a bound chord may run its command from the focused surface.
 *
 * One policy for the document listener and the terminal, so both decide alike:
 * composition and AltGr are typing; unmodified and shift-only chords never leave a
 * text field or the terminal (WCAG 2.1.4); every other bound chord fires, except the
 * native editing chords a text control owns and, off Apple, Ctrl-only chords in the terminal.
 */
export function isShortcutEligible(
  event: ShortcutKeyEvent,
  surface: ShortcutSurface,
  apple: boolean
): boolean {
  if (event.isComposing) return false;
  const modified = event.metaKey || event.ctrlKey || event.altKey;
  if (!modified) return surface === "none";
  if (!apple && event.getModifierState("AltGraph")) return false;
  if (surface === "text") return !isNativeTextChord(event, apple);
  if (surface === "terminal") return apple || !isShellControlChord(event);
  return true;
}
