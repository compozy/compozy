import type { PaletteRegistry } from "./cmd-palette-types";
import { isShortcutEligible, shortcutSurface, type ShortcutSurface } from "./shortcut-eligibility";
import {
  parseShortcutChord,
  shortcutMatches,
  type ParsedShortcutChord,
  type PrimaryShortcutModifier,
} from "./window-manager-shortcuts";

export interface PaletteKeyBinding {
  readonly commandId: string;
  readonly label: string;
  readonly section: string;
  readonly chords: readonly ParsedShortcutChord[];
  /** Survives a daemon reconnect (e.g. the cheatsheet). */
  readonly availabilityExempt: boolean;
  /** False disables the chord until the command's context comes back. */
  readonly available: boolean;
}

/**
 * The keyboard's view of the registry.
 *
 * Chords come from the daemon keymap the catalog carries, so the listener holds
 * no chord literal (BR-6) and a rebind reaches the keyboard, the palette badge,
 * the menubar and the cheatsheet in the same revision.
 */
export function paletteKeyBindings(registry: PaletteRegistry): readonly PaletteKeyBinding[] {
  const bindings: PaletteKeyBinding[] = [];
  for (const command of registry.commands) {
    const chords = command.bindings.flatMap(binding => {
      const parsed = parseShortcutChord(binding);
      return parsed === null ? [] : [parsed];
    });
    if (chords.length === 0) continue;
    bindings.push({
      commandId: command.id,
      label: command.title,
      section: command.section,
      chords,
      availabilityExempt: command.availability_exempt,
      available: command.available,
    });
  }
  return bindings;
}

/**
 * The binding a keystroke runs from the focused surface, or null.
 *
 * Matching and the eligibility policy live together so the document listener and
 * the terminal ask the same question of the same effective keymap.
 */
export function eligiblePaletteKeyBinding(
  event: KeyboardEvent,
  bindings: readonly PaletteKeyBinding[],
  primaryModifier: PrimaryShortcutModifier,
  surface: ShortcutSurface = shortcutSurface(event.target)
): PaletteKeyBinding | null {
  const binding = bindings.find(candidate =>
    candidate.chords.some(chord => shortcutMatches(event, chord, primaryModifier))
  );
  if (!binding) return null;
  return isShortcutEligible(event, surface, primaryModifier === "meta") ? binding : null;
}
