import type { OsAttentionRow, OsLayoutUpgradeAttentionRow } from "./attention-model";

const KEY = "compozy.layout-upgrade.toasted";
/** A workspace converts once per profile; this many occurrences outlast any real account. */
const REMEMBERED_LIMIT = 64;

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
    localStorage.setItem(KEY, JSON.stringify(ids.slice(-REMEMBERED_LIMIT)));
  } catch {
    /* Storage may be disabled: the toast then comes once per page load. */
  }
}

/**
 * The layout-upgrade rows this client has not been told about yet, by the bell item's occurrence
 * id, and records them as told: each occurrence is offered to the toast once per client, across
 * reloads, whether or not the toast then shows (an open bell, a muted workspace or toasts turned
 * off already account for it). `told` keeps the page's own record when storage is unavailable.
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
    writeToasted([...stored, ...untold.map(row => row.notificationId ?? row.id)]);
  }
  return untold;
}
