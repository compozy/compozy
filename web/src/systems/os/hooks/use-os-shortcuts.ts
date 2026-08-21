import { useEffect, useEffectEvent } from "react";

import { paletteKeyBindings, type PaletteKeyBinding } from "../lib/cmd-palette-keymap";
import type { PaletteRegistry } from "../lib/cmd-palette-types";
import { primaryShortcutModifier, shortcutMatches } from "../lib/window-manager-shortcuts";

export interface OsShortcutOptions {
  /** Off while a blocking surface owns the shell (first-run setup). */
  enabled?: boolean;
  /** Escape stays the one hardcoded shell key; the registry owns every other chord. */
  onEscape: () => void;
}

/** AltGr aliases ⌃⌥ on some layouts — never steal keystrokes from text entry. */
function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  const editable = target.closest("input, textarea, select, [contenteditable]");
  if (!(editable instanceof HTMLElement)) return false;
  return (
    editable.isContentEditable ||
    (editable.hasAttribute("contenteditable") &&
      editable.getAttribute("contenteditable") !== "false") ||
    editable instanceof HTMLInputElement ||
    editable instanceof HTMLTextAreaElement ||
    editable instanceof HTMLSelectElement
  );
}

function allowedInEditable(binding: PaletteKeyBinding, chordIndex: number): boolean {
  if (binding.allowInEditable) return true;
  if (binding.commandId !== "shortcuts.cheatsheet") return false;
  return binding.chords[chordIndex]?.modifiers.has("meta") ?? false;
}

/**
 * One registry-driven keydown listener.
 *
 * The chord grammar still lives in `window-manager-shortcuts`, but which
 * commands exist, what they are called, whether they are currently available
 * and what they do now all come from the daemon catalog — so a chord and its
 * palette row cannot disagree, and pressing one runs exactly what clicking the
 * other runs (ADR-006).
 */
export function useOsShortcuts(
  registry: PaletteRegistry,
  run: (commandId: string) => void,
  { enabled = true, onEscape }: OsShortcutOptions
): void {
  const handleKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.repeat) return;
    if (event.key === "Escape") {
      if (event.defaultPrevented) return;
      onEscape();
      return;
    }
    const editableTarget = isEditableTarget(event.target);
    const primaryModifier = primaryShortcutModifier(navigator.platform);
    for (const binding of paletteKeyBindings(registry)) {
      const chordIndex = binding.chords.findIndex(chord =>
        shortcutMatches(event, chord, primaryModifier)
      );
      if (chordIndex < 0) continue;
      if (editableTarget && !allowedInEditable(binding, chordIndex)) return;
      // A chord whose command is unavailable is still *bound* — swallowing the
      // keystroke is what keeps it from falling through to the browser, and the
      // seam reports the runtime's reason rather than acting.
      event.preventDefault();
      if (!binding.available && !binding.availabilityExempt) return;
      run(binding.commandId);
      return;
    }
  });

  useEffect(() => {
    if (!enabled) return undefined;
    const listener = (event: KeyboardEvent) => handleKeyDown(event);
    document.addEventListener("keydown", listener);
    return () => document.removeEventListener("keydown", listener);
  }, [enabled]);
}
