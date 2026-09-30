import type { OsAttentionRow } from "./attention-model";
import type { OsWindow } from "./os-types";

/**
 * Desktops holding a session window that needs you, so the pager can mark an
 * off-screen desktop. Stale rows count for nothing, as everywhere else.
 */
export function desktopIdsNeedingYou(
  windows: Readonly<Record<string, OsWindow>>,
  needsYou: readonly OsAttentionRow[]
): ReadonlySet<string> {
  const sessionIds = new Set(
    needsYou.flatMap(row => (row.kind === "session" && !row.stale ? [row.id] : []))
  );
  const desktopIds = new Set<string>();
  if (sessionIds.size === 0) return desktopIds;
  for (const window of Object.values(windows)) {
    if (window.app === "session" && window.instanceKey && sessionIds.has(window.instanceKey)) {
      desktopIds.add(window.desktopId);
    }
  }
  return desktopIds;
}
