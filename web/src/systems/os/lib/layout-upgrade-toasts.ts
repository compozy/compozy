import type { OsAttentionRow, OsLayoutUpgradeAttentionRow } from "./attention-model";

const KEY = "compozy.layout-upgrade.toasted";

/**
 * The stored record's bound. It stays above the bell's 100-item page, so every occurrence the list
 * can hold fits at once and a listed occurrence is never the one dropped.
 */
export const TOLD_LAYOUT_UPGRADES_CAP = 256;

function readToasted(): string[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(KEY) ?? "[]");
    return Array.isArray(parsed) ? parsed.filter(id => typeof id === "string") : [];
  } catch {
    return [];
  }
}

function writeToasted(ids: string[]): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(ids));
  } catch {
    /* Storage may be disabled: the toast then comes once per page load. */
  }
}

/**
 * Bounds the told record, oldest first. Occurrences the current list holds are dropped last, so
 * one that is listed now never toasts again; one that left the list is kept too, until the record
 * outgrows its bound, because it may be listed again (a later page, a partial read).
 */
function boundTold(told: Set<string>, listed: ReadonlySet<string>): string[] {
  const ids = [...told];
  let excess = ids.length - TOLD_LAYOUT_UPGRADES_CAP;
  if (excess <= 0) return ids;
  const kept = ids.filter(id => {
    if (excess > 0 && !listed.has(id)) {
      excess--;
      return false;
    }
    return true;
  });
  return kept.slice(Math.max(0, kept.length - TOLD_LAYOUT_UPGRADES_CAP));
}

/**
 * The layout-upgrade rows this client has not been told about yet, by the bell item's occurrence
 * id, and records them as told: each occurrence is offered to the toast once per client, across
 * reloads, whether or not the toast then shows (an open bell, a muted workspace or toasts turned
 * off already account for it). `told` keeps the page's own record when storage is unavailable.
 * An occurrence absent from one list is still remembered, so it never toasts again when a later
 * list holds it. Storage is read only when the list holds an occurrence `told` does not have yet.
 */
export function takeUntoldLayoutUpgrades(
  rows: readonly OsAttentionRow[],
  told: Set<string>
): OsLayoutUpgradeAttentionRow[] {
  // Storage is read only for an occurrence this page has not seen, not on every render.
  const unseen = rows.some(
    row => row.kind === "layout-upgrade" && !row.stale && !told.has(row.notificationId ?? row.id)
  );
  if (!unseen) return [];
  for (const id of readToasted()) told.add(id);
  const untold: OsLayoutUpgradeAttentionRow[] = [];
  const listed = new Set<string>();
  for (const row of rows) {
    if (row.kind !== "layout-upgrade") continue;
    const id = row.notificationId ?? row.id;
    listed.add(id);
    if (row.stale || told.has(id)) continue;
    told.add(id);
    untold.push(row);
  }
  if (untold.length > 0) writeToasted(boundTold(told, listed));
  return untold;
}
