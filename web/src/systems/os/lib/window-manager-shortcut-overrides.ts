/**
 * Override resolution: expanding range overrides, applying them over a base
 * keymap with the daemon's override-wins rule, and reporting collisions.
 */
import {
  parseEffectiveChords,
  parseShortcutChord,
  parseShortcutRange,
  SHORTCUT_RANGE_FAMILIES,
  SURFACE_LOCAL_CHORDS,
  type ShortcutBinding,
  type ShortcutConflict,
  type ShortcutMap,
} from "./window-manager-shortcuts";
import { withoutRetiredShortcuts } from "./window-manager-retired-shortcuts";

/**
 * Each override as member bindings. Entries run in id order, so a family range
 * (`desktop.switch`) expands before its members (`desktop.switch.1`) and an
 * explicit member binding wins its slot whatever order the config stored.
 */
export function expandShortcutOverrides(overrides: ShortcutMap): Record<string, ShortcutBinding> {
  const expanded: Record<string, ShortcutBinding> = {};
  const entries = Object.entries(overrides).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
  for (const [actionId, binding] of entries) {
    const family = SHORTCUT_RANGE_FAMILIES.find(candidate => candidate.action === actionId);
    if (!family) {
      expanded[actionId] = binding.flatMap(member => {
        const parsed = parseShortcutChord(member);
        return parsed ? [parsed.canonical] : [];
      });
      continue;
    }
    for (const raw of binding) {
      const range = parseShortcutRange(raw);
      if (!range) continue;
      for (let slot = range.start; slot <= range.end && slot <= family.max; slot += 1) {
        const memberId = `${family.action}.${slot}`;
        expanded[memberId] = [...(expanded[memberId] ?? []), `${range.prefix}${slot}`];
      }
    }
  }
  return expanded;
}

/**
 * The effective keymap: `base` with `overrides` applied, mirroring the daemon.
 *
 * An override wins the chords it names: each one leaves the base binding of
 * every action the overrides do not rebind, so a stored binding stays live when
 * a newer default ships on the same chord. Two overrides on one chord still
 * conflict (`findShortcutConflicts`).
 */
export function effectiveShortcutMap(base: ShortcutMap, overrides: ShortcutMap): ShortcutMap {
  return mergeShortcutOverrides(base, overrides, true);
}

function mergeShortcutOverrides(
  base: ShortcutMap,
  overrides: ShortcutMap,
  displaceClaimedDefaults: boolean
): ShortcutMap {
  // The daemon skips a retired binding, so its chord claims nothing.
  overrides = withoutRetiredShortcuts(overrides);
  const effective: Record<string, ShortcutBinding> = Object.fromEntries(
    Object.entries(base).map(([actionId, binding]) => [actionId, [...binding]])
  );
  for (const family of SHORTCUT_RANGE_FAMILIES) {
    if (!(family.action in overrides)) continue;
    for (let slot = 1; slot <= family.max; slot += 1) delete effective[`${family.action}.${slot}`];
  }
  const claims = new Map<string, string>();
  const overridden = new Set<string>();
  for (const [actionId, binding] of Object.entries(expandShortcutOverrides(overrides))) {
    effective[actionId] = [...binding];
    overridden.add(actionId);
    for (const chord of binding) if (!claims.has(chord)) claims.set(chord, actionId);
  }
  if (!displaceClaimedDefaults) return effective;
  for (const [actionId, binding] of Object.entries(effective)) {
    if (overridden.has(actionId)) continue;
    effective[actionId] = binding.filter(chord => {
      const canonical = parseShortcutChord(chord)?.canonical ?? chord;
      const claimer = claims.get(canonical);
      return claimer === undefined || claimer === actionId;
    });
  }
  return effective;
}

/**
 * Two commands claiming one chord, or a chord a focused surface wins locally.
 *
 * Candidates come from the effective keymap itself rather than a catalog: only
 * a bound id can conflict, and an override naming an id the registry has not
 * hydrated yet still has to be reported rather than silently ignored.
 */
export function findShortcutConflicts(
  overrides: ShortcutMap,
  defaults: ShortcutMap
): readonly ShortcutConflict[] {
  // Every claim, including a default an override would displace, so Settings
  // can show what a recorded chord takes over before it saves.
  const effective = mergeShortcutOverrides(defaults, overrides, false);
  const byChord = new Map<string, string[]>();
  for (const [actionId, bindings] of Object.entries(effective)) {
    for (const chord of parseEffectiveChords(bindings)) {
      const holders = byChord.get(chord.canonical);
      if (holders) holders.push(actionId);
      else byChord.set(chord.canonical, [actionId]);
    }
  }
  const conflicts: ShortcutConflict[] = [];
  for (const [chord, actionIds] of byChord) {
    if (actionIds.length > 1) conflicts.push({ chord, kind: "blocked", actionIds });
    const winner = SURFACE_LOCAL_CHORDS.get(chord);
    if (winner) conflicts.push({ chord, kind: "shadowed", actionIds, winner });
  }
  return conflicts;
}
