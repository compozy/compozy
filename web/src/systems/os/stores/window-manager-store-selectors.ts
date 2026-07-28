import type { LayoutGestureSession } from "../lib/layout-gesture-session";
import type { SnapTarget } from "../lib/snap-targets";
import type { WindowManagerConnectionStatus } from "../lib/window-manager-types";
import type {
  DesktopOverviewSegmentRequest,
  DesktopTransitionIntent,
  PendingWindowManagerCommand,
  WindowManagerActions,
  WindowManagerBinding,
  WindowManagerDiagnostic,
  WindowManagerOverlay,
  WindowManagerRevisionConflict,
  WindowManagerStoreState,
  WindowManagerWorkArea,
} from "./window-manager-store";

export function selectWindowManagerBinding(
  state: WindowManagerStoreState
): WindowManagerBinding | null {
  return state.binding;
}

export function selectWindowManagerConnectionStatus(
  state: WindowManagerStoreState
): WindowManagerConnectionStatus {
  return state.connectionStatus;
}

export function selectWindowManagerWorkArea(
  state: WindowManagerStoreState
): WindowManagerWorkArea | null {
  return state.workArea;
}

export function selectWindowManagerOverlay(
  state: WindowManagerStoreState
): WindowManagerOverlay | null {
  return state.activeOverlay;
}

export function selectDesktopOverviewSegmentRequest(
  state: WindowManagerStoreState
): DesktopOverviewSegmentRequest | null {
  return state.overviewSegmentRequest;
}

export function selectDesktopTransitionIntent(
  state: WindowManagerStoreState
): DesktopTransitionIntent | null {
  return state.transitionIntent;
}

export function selectWindowManagerGesture(
  state: WindowManagerStoreState
): LayoutGestureSession | null {
  return state.gesture;
}

export function selectWindowManagerGestureActive(
  state: WindowManagerStoreState,
  windowId: string
): boolean {
  return state.gesture?.status === "active" && state.gesture.source.windowId === windowId;
}

export function selectWindowManagerGesturePreview(
  state: WindowManagerStoreState
): SnapTarget | null {
  return state.gesture?.status === "active" ? state.gesture.preview : null;
}

export function selectPendingWindowManagerCommand(
  state: WindowManagerStoreState
): PendingWindowManagerCommand | null {
  return state.commandState.status === "pending" ? state.commandState.command : null;
}

export function selectWindowManagerConflict(
  state: WindowManagerStoreState
): WindowManagerRevisionConflict | null {
  return state.commandState.status === "conflict" ? state.commandState.conflict : null;
}

export function selectWindowManagerDiagnostic(
  state: WindowManagerStoreState
): WindowManagerDiagnostic | null {
  return state.commandState.diagnostic;
}

export function selectWindowManagerActions(state: WindowManagerStoreState): WindowManagerActions {
  return state.actions;
}
