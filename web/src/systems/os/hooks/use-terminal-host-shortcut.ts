import { eligiblePaletteKeyBinding, paletteKeyBindings } from "../lib/cmd-palette-keymap";
import { primaryShortcutModifier } from "../lib/window-manager-shortcuts";
import { usePaletteRegistry } from "./use-palette-registry";

/**
 * The terminal's view of the shell keymap.
 *
 * Built from the same registry and eligibility policy the document listener
 * uses, so a chord the shell runs from the terminal is exactly one the terminal
 * releases — user and workspace overrides and extension defaults included.
 */
export function useTerminalHostShortcut(): (event: KeyboardEvent) => boolean {
  const bindings = paletteKeyBindings(usePaletteRegistry());
  return event =>
    eligiblePaletteKeyBinding(
      event,
      bindings,
      primaryShortcutModifier(navigator.platform),
      "terminal"
    ) !== null;
}
