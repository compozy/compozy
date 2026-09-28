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
  const desktops: DesktopOverviewItem[] = model.desktops.map(desktop => ({
    id: desktop.id,
    name: desktop.name,
    thumbnail: (
      <DesktopLayoutThumbnail projection={desktop.projection} windows={desktop.windowRecords} />
    ),
    windows: desktop.windows,
  }));
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
      <DesktopsOverview
        open={model.overlay?.kind === "desktops-overview"}
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
