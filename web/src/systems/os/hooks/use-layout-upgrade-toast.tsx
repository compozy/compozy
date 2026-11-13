import { PanelsTopLeft } from "@compozy/ui/icons";
import { useEffect, useState } from "react";

import { Icon, toast } from "@compozy/ui";

import { ToastFrame } from "../components/attention-toast";
import type { OsAttentionSections } from "../lib/attention-model";
import { takeUntoldLayoutUpgrades } from "../lib/layout-upgrade-toasts";

const TOAST_DURATION_MS = 8_000;

export interface LayoutUpgradeToastOptions {
  sections: OsAttentionSections;
  bellOpen: boolean;
  /** Whether the attention policy shows toasts at all. */
  toasts: boolean;
  mutedWorkspaceIds: ReadonlySet<string>;
  onOpenBell: () => void;
}

/**
 * Toasts a workspace's move to tabs once, read from the bell's list rather than an edge: the item
 * is news that waits in the bell, so the toast only points at it. An occurrence the open bell
 * already shows, or one of a muted workspace, counts as told and never toasts later.
 */
export function useLayoutUpgradeToast({
  sections,
  bellOpen,
  toasts,
  mutedWorkspaceIds,
  onOpenBell,
}: LayoutUpgradeToastOptions): void {
  const [told] = useState(() => new Set<string>());
  useEffect(() => {
    const untold = takeUntoldLayoutUpgrades(sections.needsYou, told);
    if (!toasts || bellOpen) return;
    for (const row of untold) {
      if (mutedWorkspaceIds.has(row.workspaceId)) continue;
      toast.custom(
        id => (
          <ToastFrame
            testId="os-attention-toast-layout-upgrade"
            mark={
              <span
                aria-hidden="true"
                className="grid size-4.5 shrink-0 place-items-center rounded-full bg-info-tint text-info"
              >
                <Icon as={PanelsTopLeft} size="xs" />
              </span>
            }
            title={row.title}
            body={row.reason}
            meta={row.workspaceLabel}
            onActivate={() => {
              toast.dismiss(id);
              onOpenBell();
            }}
          />
        ),
        { id: `layout-upgrade:${row.notificationId ?? row.id}`, duration: TOAST_DURATION_MS }
      );
    }
  });
}
