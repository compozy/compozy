/**
 * Row derivation for the Settings shortcut table (S12).
 *
 * Rows are the registry's, not a curated list: every command the daemon reports
 * is bindable, so every command has a row — including ones this client cannot
 * run right now, which are still someone's to bind (US-022.AC-1).
 */
import {
  expandShortcutOverrides,
  shortcutSourceLabel,
  type ShortcutConflict,
  type WindowManagerSettingsSection,
  type WindowManagerShortcutMap,
} from "@/systems/os";

/** Aggregate keys the shipped presets write; each covers an indexed family. */
const RANGE_FAMILY_IDS = ["window.tab.jump", "desktop.switch"] as const;

export interface ShortcutTableRow {
  commandId: string;
  title: string;
  section: string;
  source: string;
  sourceLabel: string;
  /** Effective chords, as the daemon resolved them. */
  bindings: readonly string[];
  overridden: boolean;
  /** Ships with a chord but holds none — the loser of an overwrite. */
  unbound: boolean;
  /** An extension default the daemon withheld, stated verbatim. */
  dormantReason: string | null;
  /** A chord a focused surface claims locally. */
  shadowedReason: string | null;
}

function coveringFamily(commandId: string, overrides: WindowManagerShortcutMap): string | null {
  return (
    RANGE_FAMILY_IDS.find(
      family => commandId.startsWith(`${family}.`) && overrides[family] !== undefined
    ) ?? null
  );
}

/**
 * Drops one command out of a range override by materializing the members the
 * range still covers. Without this, resetting a member the preset bound would
 * look like it did nothing — the aggregate key would keep answering for it.
 */
export function withCommandReset(
  overrides: WindowManagerShortcutMap,
  commandId: string
): WindowManagerShortcutMap {
  const next: Record<string, readonly string[]> = { ...overrides };
  delete next[commandId];
  const family = coveringFamily(commandId, overrides);
  if (family === null) return next;
  delete next[family];
  for (const [memberId, binding] of Object.entries(
    expandShortcutOverrides({ [family]: overrides[family] ?? [] })
  )) {
    if (memberId !== commandId) next[memberId] = binding;
  }
  return next;
}

export function isCommandOverridden(
  commandId: string,
  overrides: WindowManagerShortcutMap
): boolean {
  return overrides[commandId] !== undefined || coveringFamily(commandId, overrides) !== null;
}

export function buildShortcutTableRows(
  section: WindowManagerSettingsSection,
  shadowed: readonly ShortcutConflict[]
): readonly ShortcutTableRow[] {
  const overrides = section.config.shortcuts;
  const defaults = section.config.shortcutDefaults;
  const effective = section.config.effectiveShortcuts;
  const dormant = new Map<string, string | null>();
  for (const entry of section.extensionDefaults) {
    if (entry.dormant) dormant.set(entry.commandId, entry.conflictWith);
  }
  return section.commands.map(command => {
    const bindings = effective[command.id] ?? [];
    const shadow = shadowed.find(entry => entry.actionIds.includes(command.id));
    const conflictWith = dormant.get(command.id);
    return {
      commandId: command.id,
      title: command.title,
      section: command.section,
      source: command.source,
      sourceLabel: shortcutSourceLabel(command.source),
      bindings,
      overridden: isCommandOverridden(command.id, overrides),
      unbound: bindings.length === 0 && (defaults[command.id]?.length ?? 0) > 0,
      dormantReason:
        conflictWith === undefined
          ? null
          : conflictWith === null
            ? "default unavailable"
            : `default unavailable — conflicts with ${conflictWith}`,
      shadowedReason:
        shadow === undefined
          ? null
          : `Shadowed while ${shadow.winner?.surface ?? "a local surface"} is focused — ${shadow.winner?.label ?? "its local action"} wins there.`,
    };
  });
}

export function shortcutSourceCounts(
  rows: readonly ShortcutTableRow[]
): ReadonlyMap<string, number> {
  const counts = new Map<string, number>();
  for (const row of rows) counts.set(row.source, (counts.get(row.source) ?? 0) + 1);
  return counts;
}
