/**
 * Source grouping for keyboard surfaces (`_uiux.md` S12/S13).
 *
 * The registry reports a source per command, so a group exists here only while
 * something contributes to it: an extension's rows appear the moment it loads
 * and vanish when it is disabled, with no list of known sources to maintain.
 */
import type { ShortcutCheatsheetRow } from "./window-manager-shortcuts";

export const CORE_SHORTCUT_SOURCE = "core";

export interface ShortcutSourceGroup {
  source: string;
  label: string;
  sections: ReadonlyArray<{ title: string; rows: readonly ShortcutCheatsheetRow[] }>;
}

/** Curated reading order; anything the registry adds follows, never dropped. */
export const SHORTCUT_SECTION_ORDER: readonly string[] = [
  "Shell",
  "Window",
  "Tiling",
  "Tabs",
  "Sessions",
  "Desktops",
  "Workspaces",
  "Layout",
];

/** `core` reads as the product's own areas; an extension reads as its name. */
export function shortcutSourceLabel(source: string): string {
  if (source === CORE_SHORTCUT_SOURCE) return "Core areas";
  return source.startsWith("ext.") ? source.slice("ext.".length) : source;
}

export function orderShortcutSections(present: Iterable<string>): readonly string[] {
  const unique = [...new Set(present)];
  return [
    ...SHORTCUT_SECTION_ORDER.filter(section => unique.includes(section)),
    ...unique.filter(section => !SHORTCUT_SECTION_ORDER.includes(section)),
  ];
}

/** Core first, then every contributing extension alphabetically. */
export function orderShortcutSources(present: Iterable<string>): readonly string[] {
  const unique = [...new Set(present)];
  const extensions = unique
    .filter(source => source !== CORE_SHORTCUT_SOURCE)
    .sort((left, right) => shortcutSourceLabel(left).localeCompare(shortcutSourceLabel(right)));
  return unique.includes(CORE_SHORTCUT_SOURCE) ? [CORE_SHORTCUT_SOURCE, ...extensions] : extensions;
}

export function groupShortcutRowsBySource(
  rows: readonly ShortcutCheatsheetRow[]
): readonly ShortcutSourceGroup[] {
  const groups: ShortcutSourceGroup[] = [];
  for (const source of orderShortcutSources(rows.map(row => row.source))) {
    const sourceRows = rows.filter(row => row.source === source);
    const sections = orderShortcutSections(sourceRows.map(row => row.section)).map(title => ({
      title,
      rows: sourceRows.filter(row => row.section === title),
    }));
    groups.push({ source, label: shortcutSourceLabel(source), sections });
  }
  return groups;
}
