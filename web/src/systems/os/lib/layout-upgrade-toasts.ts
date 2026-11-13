import type { OsAttentionRow, OsLayoutUpgradeAttentionRow } from "./attention-model";

const KEY = "compozy.layout-upgrade.toasted";

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
 * The layout-upgrade rows this client has not been told about yet, by the bell item's occurrence
 * id, and records them as told: each occurrence is offered to the toast once per client, across
 * reloads, whether or not the toast then shows (an open bell, a muted workspace or toasts turned
 * off already account for it). `told` keeps the page's own record when storage is unavailable.
 * The stored record keeps exactly the told occurrences the list still holds: one that left the
 * list (read, dismissed) never comes back, and one still listed is never forgotten, so the record
 * is bounded by the list rather than by a count that could drop an unread occurrence.
 */
export function takeUntoldLayoutUpgrades(
  rows: readonly OsAttentionRow[],
  told: Set<string>
): OsLayoutUpgradeAttentionRow[] {
  const stored = readToasted();
  for (const id of stored) told.add(id);
  const untold: OsLayoutUpgradeAttentionRow[] = [];
  for (const row of rows) {
    if (row.kind !== "layout-upgrade" || row.stale) continue;
    const id = row.notificationId ?? row.id;
    if (told.has(id)) continue;
    told.add(id);
    untold.push(row);
  }
  if (untold.length > 0) {
    const listed = new Set<string>();
    for (const row of rows) {
      if (row.kind === "layout-upgrade") listed.add(row.notificationId ?? row.id);
    }
    writeToasted([...told].filter(id => listed.has(id)));
  }
  return untold;
}
