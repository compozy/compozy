import type { PaletteRegistry, ResolvedPaletteCommand } from "./cmd-palette-types";

/**
 * Section assembly for the palette root and destination mode.
 *
 * Ordering here is deterministic and text-only: frecency, query learning and
 * pins arrive with the personalization slice and replace the comparator without
 * touching the section grammar. What this file already owns for good is the
 * honesty of a bounded list — a capped group either scrolls or says exactly how
 * many rows it withheld (BR-18).
 */
export const PALETTE_GROUP_CAP = 6;

/** Fixed group precedence; unknown sections keep catalog order after these. */
const SECTION_ORDER: readonly string[] = [
  "Views",
  "Shell",
  "Window",
  "Tabs",
  "Tiling",
  "Layout",
  "Sessions",
  "Desktops",
  "Workspaces",
  "Apps",
  "Settings",
];

export interface PaletteSection {
  readonly title: string;
  readonly commands: readonly ResolvedPaletteCommand[];
  /** Total matches before the cap; equal to `commands.length` when nothing was withheld. */
  readonly total: number;
}

function normalize(value: string): string {
  return value
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();
}

/** Substring match over title, id, alias and keywords; the scorer lands in P2. */
export function commandMatchesQuery(command: ResolvedPaletteCommand, query: string): boolean {
  const needle = normalize(query.trim());
  if (needle === "") return true;
  const haystack = [command.title, command.id, command.alias ?? "", ...(command.keywords ?? [])];
  return haystack.some(entry => normalize(entry).includes(needle));
}

function sectionRank(title: string): number {
  const index = SECTION_ORDER.indexOf(title);
  return index === -1 ? SECTION_ORDER.length : index;
}

/**
 * Available commands sort above disabled ones inside a group: a blocked row
 * stays visible and honest (BR-8) without pushing runnable commands out of the
 * cap.
 */
function compareCommands(left: ResolvedPaletteCommand, right: ResolvedPaletteCommand): number {
  if (left.available !== right.available) return left.available ? -1 : 1;
  return left.title.localeCompare(right.title);
}

export interface PaletteSectionInput {
  readonly registry: PaletteRegistry;
  readonly query: string;
  /**
   * Destination mode offers only what a new tab can become: navigable app
   * targets. Ineligible groups are absent, not disabled (`_uiux.md` S10).
   */
  readonly destination: boolean;
  readonly cap?: number;
}

export function assemblePaletteSections({
  registry,
  query,
  destination,
  cap = PALETTE_GROUP_CAP,
}: PaletteSectionInput): readonly PaletteSection[] {
  const grouped = new Map<string, ResolvedPaletteCommand[]>();
  for (const command of registry.commands) {
    if (destination && command.action.kind !== "navigate") continue;
    if (!commandMatchesQuery(command, query)) continue;
    const title = command.section.trim() || "Commands";
    const bucket = grouped.get(title);
    if (bucket) bucket.push(command);
    else grouped.set(title, [command]);
  }
  const sections: PaletteSection[] = [];
  for (const [title, commands] of grouped) {
    const sorted = [...commands].sort(compareCommands);
    sections.push({ title, commands: sorted.slice(0, cap), total: sorted.length });
  }
  return sections.sort((left, right) => {
    const rank = sectionRank(left.title) - sectionRank(right.title);
    return rank === 0 ? left.title.localeCompare(right.title) : rank;
  });
}

/** The exact overflow note for a capped group — never a vague "and more". */
export function overflowNote(section: PaletteSection): string | null {
  return section.total > section.commands.length
    ? `showing ${section.commands.length} of ${section.total}`
    : null;
}
