import { cn } from "@compozy/ui";
import { useState } from "react";

import type { DesktopManagerSurfacesModel } from "../hooks/use-desktop-manager-surfaces";
import { useDesktopOverviewDrag } from "../hooks/use-desktop-overview-drag";
import { useOsShell } from "../hooks/use-os-shell";
import { useTabSessions } from "../hooks/use-os-tab-bar";
import type { OsWindow } from "../lib/os-types";
import { windowManagerStore } from "../stores/window-manager-store";
import type { DesktopOverviewItem, DesktopsOverviewState } from "../lib/desktops-overview-model";
import { DesktopsOverview } from "./desktops-overview";

function overviewState(input: {
  hydration: "pending" | "live" | "degraded";
  activeDesktopId: string | null;
  desktops: readonly DesktopOverviewItem[];
  conflictMessage: string | null;
  diagnosticMessage: string | null;
}): DesktopsOverviewState {
  if (input.conflictMessage) return { status: "conflict", message: input.conflictMessage };
  if (input.hydration === "pending") return { status: "loading" };
  if (input.hydration === "degraded" && input.desktops.length === 0) {
    return {
      status: "error",
      message: input.diagnosticMessage ?? "The saved window layout is unavailable.",
    };
  }
  return {
    status: "ready",
    desktops: input.desktops,
    activeDesktopId: input.activeDesktopId,
  };
}

/**
 * Desktop management overview. Connection health and refused-command notices
 * live in the menubar's single status pill (`OsHydrationStatus`).
 */
export interface DesktopManagerSurfacesProps {
  model: DesktopManagerSurfacesModel;
  /** Desktops holding a window that needs you (the pager's projection). */
  needsYouDesktopIds: ReadonlySet<string>;
  /** Live `desktop.switch.N` chord labels by position (index 0 = ⌃1); null when unbound. */
  switchShortcuts: readonly (string | null)[];
  onCreateDesktop: () => void;
  onSwitchDesktop: (desktopId: string) => void;
  onRenameDesktop: (desktopId: string, name: string) => void;
  onReorderDesktop: (desktopId: string, order: number) => void;
  onDeleteDesktop: (desktopId: string, destinationId: string | null) => void;
  /** Moves the window's whole tab, split included, to the end of the destination's bar. */
  onMoveTab: (windowId: string, destinationDesktopId: string) => void;
  onOpenChange: (open: boolean) => void;
  onRetry: () => void;
  onResolveConflict: () => void;
  /** Places the overlay host in the shell grid: the area the overview covers. */
  className?: string;
}

export function DesktopManagerSurfaces({
  model,
  needsYouDesktopIds,
  switchShortcuts,
  onCreateDesktop,
  onSwitchDesktop,
  onRenameDesktop,
  onReorderDesktop,
  onDeleteDesktop,
  onMoveTab,
  onOpenChange,
  onRetry,
  onResolveConflict,
  className,
}: DesktopManagerSurfacesProps) {
  // The overview portals into this host, which the shell places over the tab bar and the desk;
  // the menubar and the rail stay in place.
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);
  const { coordinator } = useOsShell();
  const { startOverviewTabDrag, dropTargetDesktopId, dropSurface, dragKind } =
    useDesktopOverviewDrag();
  const desktops: DesktopOverviewItem[] = model.desktops.map((desktop, index) => ({
    ...desktop,
    needsYou: needsYouDesktopIds.has(desktop.id),
    switchShortcut: switchShortcuts[index] ?? null,
  }));
  const hasSession = desktops.some(desktop =>
    desktop.tabs.some(tab => tab.members.some(member => member.app === "session"))
  );
  const sessionById = useTabSessions(hasSession);
  const sessionOf = (win: OsWindow) =>
    win.app === "session" && win.instanceKey ? sessionById.get(win.instanceKey) : undefined;
  const overview = overviewState({
    hydration: model.hydration,
    activeDesktopId: model.activeDesktopId,
    desktops,
    conflictMessage: model.conflict
      ? "Your desktops changed somewhere else. Reload to see the latest."
      : null,
    diagnosticMessage: model.diagnostic?.message ?? null,
  });

  return (
    <>
      <div
        ref={setOverlayHost}
        data-slot="os-desk-overlays"
        // contain-paint makes the portaled overview resolve against this area.
        className={cn(
          "pointer-events-none relative z-40 min-h-0 min-w-0 contain-paint [&:not(:empty)]:pointer-events-auto",
          className
        )}
      />
      <DesktopsOverview
        open={model.overlay?.kind === "desktops-overview"}
        container={overlayHost}
        state={overview}
        initialFocusSegment={model.overviewSegmentRequest}
        desktopAction={model.overviewDesktopAction}
        onDesktopActionApplied={() => windowManagerStore.trigger.overviewDesktopActionConsumed()}
        dragKind={dragKind}
        dropSurface={dropSurface}
        busy={model.pending !== null}
        canMutate={model.canMutate}
        onOpenChange={onOpenChange}
        onCreateDesktop={onCreateDesktop}
        onSwitchDesktop={onSwitchDesktop}
        onRenameDesktop={onRenameDesktop}
        onReorderDesktop={onReorderDesktop}
        onDeleteDesktop={onDeleteDesktop}
        onMoveTab={onMoveTab}
        // Choosing a tab switches to its desktop and shows it there (AC-5): focusing its lead
        // window does both for this client, as a tab click in the bar does.
        onChooseTab={(desktopId, windowId) => {
          if (desktopId !== model.activeDesktopId) onSwitchDesktop(desktopId);
          void coordinator.userFocus(windowId);
        }}
        onTabPointerDown={startOverviewTabDrag}
        dropTargetDesktopId={dropTargetDesktopId}
        sessionOf={sessionOf}
        onRetry={onRetry}
        onResolveConflict={onResolveConflict}
      />
    </>
  );
}
