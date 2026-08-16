import { useRef } from "react";
import { shallowEqual } from "@xstate/store";

import { frameSeamEdits } from "../lib/frame-seams";
import type { ProjectedFrameSeam, ProjectedSeam } from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";
import type { DesktopOverviewSegmentRequest } from "../stores/window-manager-store-types";
import { useDesktop } from "./use-desktop";
import { useDesktopManagerSurfaces } from "./use-desktop-manager-surfaces";
import { useDesktopOverlays } from "./use-desktop-overlays";
import { useDesktopShellState } from "./use-desktop-shell-state";
import type { DesktopShellModel } from "./use-desktop-shell-model";
import { useDesktopTransitionIntent } from "./use-window-manager-store";
import { useAttentionNotifier } from "./use-attention-notifier";
import { useDocumentTitleBadge } from "./use-document-title-badge";
import { useFocusedSessionId } from "./use-focused-session-id";
import { useOsAttention } from "./use-os-attention";
import { useOsReducedMotion } from "./use-os-reduced-motion";
import { useOsShell } from "./use-os-shell";
import { useOsShortcuts } from "./use-os-shortcuts";
import { useOsWinLayer } from "./use-os-win-layer";

export interface DesktopShellBodyOptions {
  /** First-run setup owns the shell; the chrome is inert and shortcuts are off. */
  firstRun?: boolean;
  onNewSession: () => void;
}

function setSeamPreview(seam: ProjectedSeam, deltaPx: number): void {
  windowManagerStore.trigger.seamPreviewSet({
    preview: {
      kind: "split",
      splitId: seam.splitId,
      boundaryIndex: seam.boundaryIndex,
      deltaPx,
    },
  });
}

function setFrameSeamPreview(seam: ProjectedFrameSeam, deltaPx: number): void {
  windowManagerStore.trigger.seamPreviewSet({
    preview: { kind: "frame", seamId: seam.id, deltaPx },
  });
}

function clearSeamPreview(): void {
  windowManagerStore.trigger.seamPreviewCleared();
}

function completeDesktopTransition(): void {
  windowManagerStore.trigger.transitionIntentChanged({ intent: null });
}

function setDesktopManagerOpen(open: boolean): void {
  if (open) {
    windowManagerStore.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } });
    return;
  }
  windowManagerStore.trigger.overlayClosed();
}

function openDesktopOverview(request: DesktopOverviewSegmentRequest): void {
  windowManagerStore.trigger.overviewSegmentRequested({ request });
}

/** Composes the live runtime models consumed by the presentational desktop shell body. */
export function useDesktopShellBody(model: DesktopShellModel, options: DesktopShellBodyOptions) {
  const firstRun = options.firstRun ?? false;
  const desktopRef = useRef<HTMLDivElement>(null);
  const desktop = useDesktopShellState();
  const overlays = useDesktopOverlays();
  const attention = useOsAttention(model.runtimeWorkspace, model.sessionCatalogStreamStatus);
  useDocumentTitleBadge(attention.notificationCount);
  useAttentionNotifier({
    sections: attention.sections,
    streamLive: model.sessionCatalogStreamStatus === "live",
    focusedSessionId: useFocusedSessionId(),
    onOpenBell: () => overlays.setOverlayOpen("bell", true),
  });
  const managerSurfaces = useDesktopManagerSurfaces();
  const winLayer = useOsWinLayer();
  const reducedMotion = useOsReducedMotion();
  const transition = useDesktopTransitionIntent();
  const { manager } = useOsShell();
  const pager = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      desktops: state.desktops,
      compact: state.presentation === "compact",
      canSwitchDesktop:
        state.client !== null &&
        state.hydration === "live" &&
        state.connectionStatus === "connected",
    }),
    shallowEqual
  );

  useOsShortcuts(
    {
      onPalette: () => overlays.toggleOverlay("palette"),
      onNewSession: options.onNewSession,
      onDesktops: () => overlays.toggleOverlay("desktops"),
      onWorkspaces: () => overlays.toggleOverlay("workspaces"),
      onToggleGlobalScope: model.toggleGlobalScope,
      onEscape: () => {
        if (overlays.activeOverlay !== null) return;
        if (document.querySelector('[data-slot="dialog-content"]')) return;
        desktopRef.current?.focus();
      },
    },
    { enabled: !firstRun }
  );

  const onResize = (splitId: string, boundaryIndex: number, delta: number) => {
    // Clear the live seam preview only after the resize reconciles, so panes go
    // straight from previewed to committed sizes.
    void manager.resizeLayout(splitId, boundaryIndex, delta).completion.finally(clearSeamPreview);
  };
  const onFrameResize = (seam: ProjectedFrameSeam, deltaPx: number) => {
    const snapshot = manager.getState().snapshot;
    const groups = snapshot?.desktops.find(entry => entry.id === seam.desktopId)?.groups;
    const edits = groups === undefined ? [] : frameSeamEdits(groups, seam, deltaPx);
    if (edits.length === 0) {
      clearSeamPreview();
      return;
    }
    void manager.resizeGroupFrames(seam.desktopId, edits).completion.finally(clearSeamPreview);
  };
  return {
    attention,
    desktop,
    desktopRef,
    manager,
    managerSurfaces,
    onResize,
    onFrameResize,
    onDesktopManagerOpenChange: setDesktopManagerOpen,
    onOpenDesktopOverview: openDesktopOverview,
    onSeamPreview: setSeamPreview,
    onFrameSeamPreview: setFrameSeamPreview,
    onSeamPreviewEnd: clearSeamPreview,
    onTransitionComplete: completeDesktopTransition,
    overlays,
    pager,
    reducedMotion,
    transition,
    winLayer,
  };
}
