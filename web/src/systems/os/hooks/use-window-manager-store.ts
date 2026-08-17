import { useSelector } from "@xstate/store-react";

import type { PaletteViewFrame } from "../lib/palette-view-stack";
import type { SnapTarget } from "../lib/snap-targets";
import {
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
  selectWindowManagerConflict,
  selectWindowManagerDiagnostic,
  selectWindowManagerGestureActive,
  selectWindowManagerGestureDragging,
  selectWindowManagerGesturePreview,
  selectWindowManagerOverlay,
  selectWindowManagerWorkArea,
  type DesktopOverviewSegmentRequest,
  type DesktopTransitionIntent,
  type PendingWindowManagerCommand,
  type WindowManagerDiagnostic,
  type WindowManagerOverlay,
  type WindowManagerRevisionConflict,
  type WindowManagerWorkArea,
  type WindowPaletteIntent,
  windowManagerStore,
} from "../stores/window-manager-store";

export function useWindowManagerWorkArea(): WindowManagerWorkArea | null {
  return useSelector(windowManagerStore, snapshot => selectWindowManagerWorkArea(snapshot.context));
}

export function useWindowManagerOverlay(): WindowManagerOverlay | null {
  return useSelector(windowManagerStore, snapshot => selectWindowManagerOverlay(snapshot.context));
}

export function useDesktopOverviewSegmentRequest(): DesktopOverviewSegmentRequest | null {
  return useSelector(windowManagerStore, snapshot =>
    selectDesktopOverviewSegmentRequest(snapshot.context)
  );
}

export function useDesktopTransitionIntent(): DesktopTransitionIntent | null {
  return useSelector(windowManagerStore, snapshot =>
    selectDesktopTransitionIntent(snapshot.context)
  );
}

export function useWindowManagerGestureActive(windowId: string): boolean {
  return useSelector(windowManagerStore, snapshot =>
    selectWindowManagerGestureActive(snapshot.context, windowId)
  );
}

export function useWindowManagerGestureDragging(windowId: string): boolean {
  return useSelector(windowManagerStore, snapshot =>
    selectWindowManagerGestureDragging(snapshot.context, windowId)
  );
}

export function useWindowManagerGesturePreview(): SnapTarget | null {
  return useSelector(windowManagerStore, snapshot =>
    selectWindowManagerGesturePreview(snapshot.context)
  );
}

export function usePendingWindowManagerCommand(): PendingWindowManagerCommand | null {
  return useSelector(windowManagerStore, snapshot =>
    selectPendingWindowManagerCommand(snapshot.context)
  );
}

export function useWindowManagerConflict(): WindowManagerRevisionConflict | null {
  return useSelector(windowManagerStore, snapshot => selectWindowManagerConflict(snapshot.context));
}

export function useWindowManagerDiagnostic(): WindowManagerDiagnostic | null {
  return useSelector(windowManagerStore, snapshot =>
    selectWindowManagerDiagnostic(snapshot.context)
  );
}

export function useWindowPaletteIntent(): WindowPaletteIntent | null {
  return useSelector(windowManagerStore, snapshot => snapshot.context.paletteIntent);
}

export function useWindowPaletteViewStack(): readonly PaletteViewFrame[] {
  return useSelector(windowManagerStore, snapshot => snapshot.context.paletteViewStack);
}
