import type {
  BeginLayoutGestureInput,
  FinishLayoutGestureInput,
  GestureCancelReason,
  GestureDecision,
  LayoutGestureSession,
} from "../lib/layout-gesture-session";
import type { OsWindowRoute } from "../lib/os-types";
import type { SeamPreview } from "../lib/seam-preview";
import type { SnapCorner, SnapSide, SnapTarget } from "../lib/snap-targets";
import type {
  DesktopId,
  LayoutRevision,
  PixelPoint,
  PixelRect,
  WindowManagerCommandId,
  WindowManagerConnectionStatus,
} from "../lib/window-manager-types";

export interface WindowManagerBinding {
  readonly workspaceId: string;
  readonly clientId: string;
}

export interface WindowManagerWorkArea {
  readonly rect: PixelRect;
  readonly origin: PixelPoint;
}

export type WindowManagerOverlay =
  | { kind: "desktops-overview" }
  | { kind: "layout-editor"; desktopId: DesktopId | null };

export interface DesktopOverviewSegmentRequest {
  readonly direction: "earlier" | "later";
  readonly hiddenDesktopIds: readonly DesktopId[];
  readonly anchorDesktopId: DesktopId;
}

export interface DesktopTransitionIntent {
  readonly fromDesktopId: DesktopId;
  readonly toDesktopId: DesktopId;
  readonly direction: "earlier" | "later";
  readonly mode: "slide" | "crossfade" | "instant";
}

export interface WindowRouteIntent {
  readonly id: string;
  readonly windowId: string;
  readonly route: OsWindowRoute;
}

export interface WindowPlacementCycle {
  readonly edge: SnapSide | SnapCorner;
  readonly nextStep: number;
}

export interface PendingWindowManagerCommand {
  readonly id: string;
  readonly kind: WindowManagerCommandId;
  readonly expectedRevision: LayoutRevision;
}

export interface WindowManagerRevisionConflict {
  readonly commandId: string;
  readonly expectedRevision: LayoutRevision;
  readonly currentRevision: LayoutRevision;
}

export interface WindowManagerDiagnostic {
  readonly code: string;
  readonly message: string;
  readonly severity: "info" | "warning" | "error";
  readonly field: string | null;
}

export type WindowManagerCommandState =
  | { status: "idle"; diagnostic: WindowManagerDiagnostic | null }
  | {
      status: "pending";
      command: PendingWindowManagerCommand;
      diagnostic: WindowManagerDiagnostic | null;
    }
  | {
      status: "conflict";
      conflict: WindowManagerRevisionConflict;
      diagnostic: WindowManagerDiagnostic;
    };

/** Data-only browser interaction context. Query remains the snapshot authority. */
export interface WindowManagerStoreState {
  readonly binding: WindowManagerBinding | null;
  readonly connectionStatus: WindowManagerConnectionStatus;
  readonly workArea: WindowManagerWorkArea | null;
  readonly activeOverlay: WindowManagerOverlay | null;
  readonly overviewSegmentRequest: DesktopOverviewSegmentRequest | null;
  readonly transitionIntent: DesktopTransitionIntent | null;
  readonly routeIntents: Readonly<Record<string, WindowRouteIntent>>;
  readonly placementCycles: Readonly<Record<string, WindowPlacementCycle>>;
  readonly gesture: LayoutGestureSession | null;
  readonly seamPreview: SeamPreview | null;
  readonly commandState: WindowManagerCommandState;
}

export type WindowManagerStoreEvents = {
  bindingBound: { binding: WindowManagerBinding };
  bindingUnbound: {};
  connectionStatusChanged: { status: WindowManagerConnectionStatus };
  workAreaMeasured: { workArea: WindowManagerWorkArea | null };
  overlayOpened: { overlay: WindowManagerOverlay };
  overlayClosed: {};
  overviewSegmentRequested: { request: DesktopOverviewSegmentRequest };
  desktopStateObserved: {
    activeDesktopId: string;
    reconciledIntent: DesktopTransitionIntent | null;
  };
  transitionIntentChanged: { intent: DesktopTransitionIntent | null };
  transitionIntentRejected: { binding: WindowManagerBinding; toDesktopId: string };
  routeIntentSet: { intent: WindowRouteIntent };
  routeIntentCleared: { windowId: string; intentId: string };
  placementCycleAdvanced: { windowId: string; edge: SnapSide | SnapCorner };
  placementTargetTracked: { windowId: string; edge: SnapSide | SnapCorner | null };
  seamPreviewSet: { preview: SeamPreview };
  seamPreviewCleared: {};
  gestureBegan: BeginLayoutGestureInput;
  gesturePreviewed: {
    point: PixelPoint;
    preview: SnapTarget | null;
    currentWorkArea: PixelRect;
  };
  gestureCancelled: { reason: GestureCancelReason; point?: PixelPoint };
  gestureFinished: FinishLayoutGestureInput;
  gestureCleared: {};
  commandBegan: { command: PendingWindowManagerCommand };
  commandCompleted: {
    commandId: string;
    diagnostic?: WindowManagerDiagnostic;
    binding?: WindowManagerBinding;
  };
  commandFailed: {
    commandId: string;
    diagnostic: WindowManagerDiagnostic;
    binding?: WindowManagerBinding;
  };
  conflictRecorded: {
    conflict: WindowManagerRevisionConflict;
    diagnostic: WindowManagerDiagnostic;
    binding?: WindowManagerBinding;
  };
  conflictCleared: {};
  diagnosticReported: { diagnostic: WindowManagerDiagnostic };
  diagnosticCleared: {};
};

export type WindowManagerStoreEmitted = {
  commandAccepted: { commandId: string };
  gestureDecisionResolved: { decision: GestureDecision };
  placementCycleResolved: {
    windowId: string;
    edge: SnapSide | SnapCorner;
    cycleStep: number;
  };
};
