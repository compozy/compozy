import { useState } from "react";

import type { DesktopManagerSurfacesModel } from "../hooks/use-desktop-manager-surfaces";
import { DesktopLayoutThumbnail } from "./desktop-layout-thumbnail";
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
  onMoveWindow: (windowId: string, destinationDesktopId: string) => void;
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
  onMoveWindow,
  onOpenChange,
  onRetry,
  onResolveConflict,
}: DesktopManagerSurfacesProps) {
  // The overview portals into this desk-sized host so it covers the desk only.
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);
  const desktops: DesktopOverviewItem[] = model.desktops.map((desktop, index) => {
    const area = desktop.projection?.workArea;
    return {
      id: desktop.id,
      name: desktop.name,
      thumbnail: (
        <DesktopLayoutThumbnail projection={desktop.projection} windows={desktop.windowRecords} />
      ),
      aspectRatio: area && area.w > 0 && area.h > 0 ? area.w / area.h : undefined,
      windows: desktop.windows,
      needsYou: needsYouDesktopIds.has(desktop.id),
      switchShortcut: switchShortcuts[index] ?? null,
    };
  });
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
        busy={model.pending !== null}
        canMutate={model.canMutate}
        onOpenChange={onOpenChange}
        onCreateDesktop={onCreateDesktop}
        onSwitchDesktop={onSwitchDesktop}
        onRenameDesktop={onRenameDesktop}
        onReorderDesktop={onReorderDesktop}
        onDeleteDesktop={onDeleteDesktop}
        onMoveWindow={(windowId, _sourceDesktopId, destinationDesktopId) =>
          onMoveWindow(windowId, destinationDesktopId)
        }
        onRetry={onRetry}
        onResolveConflict={onResolveConflict}
      />
    </>
  );
}
