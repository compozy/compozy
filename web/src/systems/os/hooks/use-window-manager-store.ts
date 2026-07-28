import { useStore } from "zustand";

import type { SnapTarget } from "../lib/snap-targets";
import {
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
  selectWindowManagerActions,
  selectWindowManagerConflict,
  selectWindowManagerDiagnostic,
  selectWindowManagerGestureActive,
  selectWindowManagerGesturePreview,
  selectWindowManagerOverlay,
  selectWindowManagerWorkArea,
  type DesktopOverviewSegmentRequest,
  type DesktopTransitionIntent,
  type PendingWindowManagerCommand,
  type WindowManagerActions,
  type WindowManagerDiagnostic,
  type WindowManagerOverlay,
  type WindowManagerRevisionConflict,
  type WindowManagerWorkArea,
  windowManagerStore,
} from "../stores/window-manager-store";

export function useWindowManagerWorkArea(): WindowManagerWorkArea | null {
  return useStore(windowManagerStore, selectWindowManagerWorkArea);
}

export function useWindowManagerOverlay(): WindowManagerOverlay | null {
  return useStore(windowManagerStore, selectWindowManagerOverlay);
}

export function useDesktopOverviewSegmentRequest(): DesktopOverviewSegmentRequest | null {
  return useStore(windowManagerStore, selectDesktopOverviewSegmentRequest);
}

export function useDesktopTransitionIntent(): DesktopTransitionIntent | null {
  return useStore(windowManagerStore, selectDesktopTransitionIntent);
}

export function useWindowManagerGestureActive(windowId: string): boolean {
  return useStore(windowManagerStore, state => selectWindowManagerGestureActive(state, windowId));
}

export function useWindowManagerGesturePreview(): SnapTarget | null {
  return useStore(windowManagerStore, selectWindowManagerGesturePreview);
}

export function usePendingWindowManagerCommand(): PendingWindowManagerCommand | null {
  return useStore(windowManagerStore, selectPendingWindowManagerCommand);
}

export function useWindowManagerConflict(): WindowManagerRevisionConflict | null {
  return useStore(windowManagerStore, selectWindowManagerConflict);
}

export function useWindowManagerDiagnostic(): WindowManagerDiagnostic | null {
  return useStore(windowManagerStore, selectWindowManagerDiagnostic);
}

export function useWindowManagerActions(): WindowManagerActions {
  return useStore(windowManagerStore, selectWindowManagerActions);
}
