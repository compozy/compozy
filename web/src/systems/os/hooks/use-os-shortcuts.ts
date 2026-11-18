import { useEffect, useEffectEvent } from "react";

import { eligiblePaletteKeyBinding, paletteKeyBindings } from "../lib/cmd-palette-keymap";
import type { PaletteRegistry } from "../lib/cmd-palette-types";
import { primaryShortcutModifier } from "../lib/window-manager-shortcuts";

export interface OsShortcutOptions {
  /** Off while a blocking surface owns the shell (first-run setup). */
  enabled?: boolean;
  /** Escape stays the one hardcoded shell key; the registry owns every other chord. */
  onEscape: () => void;
}

/**
 * One registry-driven keydown listener.
 *
 * The chord grammar still lives in `window-manager-shortcuts`, but which
 * commands exist, what they are called, whether they are currently available
 * and what they do now all come from the daemon catalog — so a chord and its
 * palette row cannot disagree, and pressing one runs exactly what clicking the
 * other runs (ADR-006). Which surface may fire a chord is
 * `shortcut-eligibility`'s single policy, shared with the terminal.
 */
export function useOsShortcuts(
  registry: PaletteRegistry,
  run: (commandId: string) => void,
  { enabled = true, onEscape }: OsShortcutOptions
): void {
  const handleKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.defaultPrevented) return;
    if (event.key === "Escape") {
      if (!event.repeat) onEscape();
      return;
    }
    const binding = eligiblePaletteKeyBinding(
      event,
      paletteKeyBindings(registry),
      primaryShortcutModifier(navigator.platform)
    );
    if (!binding) return;
    // A chord whose command is unavailable is still *bound* — swallowing the
    // keystroke is what keeps it from falling through to the browser, and the
    // seam reports the runtime's reason rather than acting. A held chord is
    // swallowed too, so holding ⌥H at the edge types nothing.
    event.preventDefault();
    if (event.repeat) return;
    if (!binding.available && !binding.availabilityExempt) return;
    run(binding.commandId);
  });

  useEffect(() => {
    if (!enabled) return undefined;
    const listener = (event: KeyboardEvent) => handleKeyDown(event);
    document.addEventListener("keydown", listener);
    return () => document.removeEventListener("keydown", listener);
  }, [enabled]);
}
