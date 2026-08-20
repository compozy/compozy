import type { PaletteRegistry } from "./cmd-palette-types";
import type { ShortcutActionDefinition } from "./window-manager-shortcuts";

/**
 * The registry as the chord layer sees it: id, label, section.
 *
 * The cheatsheet and the settings shortcut table both read this, so they list
 * exactly the commands the palette lists, under the same names — the third and
 * fourth copies of the keymap that used to exist in TypeScript are gone
 * (BR-6, US-001.AC-4).
 */
export function registryShortcutActions(
  registry: PaletteRegistry
): readonly ShortcutActionDefinition[] {
  return registry.commands.map(command => ({
    id: command.id,
    label: command.title,
    section: command.section,
    source: command.source,
    alias: command.alias,
  }));
}
