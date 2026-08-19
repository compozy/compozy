import type { PaletteRegistry } from "./cmd-palette-types";
import { parseShortcutChord, type ParsedShortcutChord } from "./window-manager-shortcuts";

/**
 * Commands that keep firing while a text control has focus.
 *
 * This is an input policy of the client, not catalog metadata: the daemon has
 * no opinion about which chords a browser text field should swallow, and
 * WCAG 2.1.4 makes the guard the client's obligation. Every id here is one the
 * operator expects to work mid-typing — window and tab lifecycle, and starting
 * a session.
 */
const ALLOWED_IN_EDITABLE: ReadonlySet<string> = new Set([
  "palette.open",
  "session.new",
  "window.close",
  "window.tab.new",
  "window.tab.next",
  "window.tab.previous",
  "window.tab.last",
  "window.tab.reopen",
  "window.tab.jump.1",
  "window.tab.jump.2",
  "window.tab.jump.3",
  "window.tab.jump.4",
  "window.tab.jump.5",
  "window.tab.jump.6",
  "window.tab.jump.7",
  "window.tab.jump.8",
]);

export interface PaletteKeyBinding {
  readonly commandId: string;
  readonly label: string;
  readonly section: string;
  readonly chords: readonly ParsedShortcutChord[];
  /** Survives a daemon reconnect (e.g. the cheatsheet). */
  readonly availabilityExempt: boolean;
  readonly allowInEditable: boolean;
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
      allowInEditable: ALLOWED_IN_EDITABLE.has(command.id),
      available: command.available,
    });
  }
  return bindings;
}

/**
 * The cheatsheet's grouping key. `window.tab.jump.3` and its siblings collapse
 * into one family row so a nine-member range does not read as nine bindings.
 */
export function paletteBindingFamily(commandId: string): string | null {
  if (commandId.startsWith("window.tab.jump.")) return "window.tab.jump";
  if (/^desktop\.switch\.\d$/.test(commandId)) return "desktop.switch";
  return null;
}
