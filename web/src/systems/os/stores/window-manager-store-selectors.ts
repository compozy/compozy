import type { WindowManagerConnectionStatus } from "../lib/window-manager-types";
import type {
  DesktopOverviewDesktopAction,
  DesktopOverviewSegmentRequest,
  DesktopTransitionIntent,
  PendingWindowManagerCommand,
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

export function selectDesktopOverviewDesktopAction(
  state: WindowManagerStoreState
): DesktopOverviewDesktopAction | null {
  return state.overviewDesktopAction;
}

export function selectDesktopTransitionIntent(
  state: WindowManagerStoreState
): DesktopTransitionIntent | null {
  return state.transitionIntent;
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
