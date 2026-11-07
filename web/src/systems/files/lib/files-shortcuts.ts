import { primaryShortcutModifier } from "@/systems/os";

type KeyChord = Pick<KeyboardEvent, "key" | "code" | "metaKey" | "ctrlKey" | "shiftKey" | "altKey">;

/** Cmd on Apple platforms, Ctrl elsewhere, with no other primary modifier held. */
function primaryHeld(event: KeyChord): boolean {
  const platform = typeof navigator === "undefined" ? "" : navigator.platform;
  return primaryShortcutModifier(platform) === "meta"
    ? event.metaKey && !event.ctrlKey
    : event.ctrlKey && !event.metaKey;
}

function keyIs(event: KeyChord, letter: string): boolean {
  return event.code === `Key${letter.toUpperCase()}` || event.key.toLowerCase() === letter;
}

/** Cmd/Ctrl+P: Go to file (explorer-scoped). */
export function isGoToFileChord(event: KeyChord): boolean {
  return primaryHeld(event) && !event.shiftKey && !event.altKey && keyIs(event, "p");
}

/** Cmd/Ctrl+Shift+F: Search file contents (explorer-scoped). */
export function isSearchChord(event: KeyChord): boolean {
  return primaryHeld(event) && event.shiftKey && !event.altKey && keyIs(event, "f");
}

/** Cmd/Ctrl+F: Find in file (viewer-scoped). */
export function isFindChord(event: KeyChord): boolean {
  return primaryHeld(event) && !event.shiftKey && !event.altKey && keyIs(event, "f");
}
