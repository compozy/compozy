import { useState } from "react";

import type { DesktopManagerSurfacesModel } from "../hooks/use-desktop-manager-surfaces";
import { useOsShell } from "../hooks/use-os-shell";
import { useTabSessions } from "../hooks/use-os-tab-bar";
import {
  useOsTabDragDesktopTarget,
  useOsTabDragSourceDesktop,
  useOsTabDragStart,
} from "../hooks/use-os-tab-drag";
import type { OsWindow } from "../lib/os-types";
import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopsOverviewState,
} from "./desktops-overview";

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
}: DesktopManagerSurfacesProps) {
  // The overview portals into this desk-sized host so it covers the desk only.
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);
  const { coordinator } = useOsShell();
  const { startOverviewTabDrag } = useOsTabDragStart();
  const dropTargetDesktopId = useOsTabDragDesktopTarget();
  // While a tab is dragged the overview is a drop surface: never modal, so it cannot trap focus,
  // make the dragged tab inert or take the release as an outside press.
  const dropSurface = useOsTabDragSourceDesktop() !== null;
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
        // contain-paint makes the portaled overview resolve against the desk.
        className="contain-paint pointer-events-none absolute inset-0 z-40 [&:not(:empty)]:pointer-events-auto"
      />
      <DesktopsOverview
        open={model.overlay?.kind === "desktops-overview"}
        container={overlayHost}
        state={overview}
        initialFocusSegment={model.overviewSegmentRequest}
        desktopAction={model.overviewDesktopAction}
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
