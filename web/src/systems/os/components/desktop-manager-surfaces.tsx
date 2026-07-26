import { AlertTriangle, WifiOff } from "lucide-react";

import { cn } from "@agh/ui";

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
      message: input.diagnosticMessage ?? "The daemon window layout is unavailable.",
    };
  }
  return {
    status: "ready",
    desktops: input.desktops,
    activeDesktopId: input.activeDesktopId,
  };
}

/** Management overview and honest daemon-connection feedback. */
export interface DesktopManagerSurfacesProps {
  model: DesktopManagerSurfacesModel;
  /** No workspace is bound, so there is no layout stream to report on. */
  unbound?: boolean;
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
  unbound = false,
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
    purpose: desktop.purpose,
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
      ? `Revision ${model.conflict.expectedRevision} is stale; the daemon is at revision ${model.conflict.currentRevision}.`
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
      {!unbound && model.hydration !== "pending" && model.connectionStatus !== "connected" ? (
        <div
          role="status"
          className={cn(
            "pointer-events-none absolute top-2 right-2 z-30 flex items-center gap-1.5",
            "rounded-pill border border-line-strong bg-shell-glass px-2.5 py-1",
            "text-form-hint text-muted backdrop-blur-shell"
          )}
        >
          {model.diagnostic ? (
            <AlertTriangle aria-hidden="true" className="size-3 text-warning" />
          ) : (
            <WifiOff aria-hidden="true" className="size-3 text-subtle" />
          )}
          {model.diagnostic?.message ??
            (model.connectionStatus === "reconnecting"
              ? "Layout reconnecting"
              : "Live layout disconnected")}
        </div>
      ) : null}
    </>
  );
}
