import { useEffect, useState } from "react";

import type { OsAttentionSections, OsLayoutUpgradeAttentionRow } from "../lib/attention-model";
import type { PresentAttentionToast } from "../lib/attention-toast-request";
import { takeUntoldLayoutUpgrades } from "../lib/layout-upgrade-toasts";

/** The bell's own copy for a conversion with no counts, which also stands for several at once. */
const COALESCED_BODY = "Floating and minimized windows are tabs now.";

export interface LayoutUpgradeToastOptions {
  sections: OsAttentionSections;
  bellOpen: boolean;
  /** Whether the attention policy shows toasts at all. */
  toasts: boolean;
  mutedWorkspaceIds: ReadonlySet<string>;
  onOpenBell: () => void;
  presentToast: PresentAttentionToast;
}

function occurrenceOf(row: OsLayoutUpgradeAttentionRow): string {
  return row.notificationId ?? row.id;
}

/**
 * Toasts a workspace's move to tabs once, read from the bell's list rather than an edge: the item
 * is news that waits in the bell, so the toast only points at it. An occurrence the open bell
 * already shows, or one of a muted workspace, counts as told and never toasts later. Occurrences
 * that arrive together (a client opening after an upgrade over several workspaces) share one
 * toast naming their workspaces, so the burst never stacks identical toasts.
 */
export function useLayoutUpgradeToast({
  sections,
  bellOpen,
  toasts,
  mutedWorkspaceIds,
  onOpenBell,
  presentToast,
}: LayoutUpgradeToastOptions): void {
  const [told] = useState(() => new Set<string>());
  useEffect(() => {
    const untold = takeUntoldLayoutUpgrades([...sections.needsYou, ...sections.finished], told);
    if (!toasts || bellOpen) return;
    const shown = untold.filter(row => !mutedWorkspaceIds.has(row.workspaceId));
    const [first] = shown;
    if (!first) return;
    const single = shown.length === 1;
    // react-doctor-disable-next-line react-doctor/no-pass-data-to-parent, react-doctor/no-pass-live-state-to-parent -- presentToast is the components layer's imperative toast sink, not parent state: raising a toast once per untold occurrence is this effect's side effect, and nothing re-renders from it.
    presentToast({
      kind: "layout-upgrade",
      id: `layout-upgrade:${shown.map(occurrenceOf).join(",")}`,
      title: first.title,
      body: single ? first.reason : COALESCED_BODY,
      meta: [...new Set(shown.map(row => row.workspaceLabel))].join(" · "),
      onActivate: onOpenBell,
    });
  });
}
