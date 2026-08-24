import type {
  BeginLayoutGestureInput,
  FinishLayoutGestureInput,
  GestureCancelReason,
  GestureDecision,
  LayoutGestureSession,
} from "../lib/layout-gesture-session";
import type { OsWindowRoute } from "../lib/os-types";
import type { PaletteViewId } from "../lib/palette-view-registry";
import type { PaletteViewFrame } from "../lib/palette-view-stack";
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
  /** The profile whose desks this client is presenting (US-026). */
  readonly profileId: string;
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

/**
 * Shell-palette request raised from inside a window (new-tab picker, deck `+`).
 * The palette consumes it as a destination scope for that window (US-005).
 */
export interface WindowPaletteIntent {
  readonly kind: "destination";
  readonly windowId: string;
}

/**
 * A deck advertising itself as the drop target of the active window drag
 * (US-014): the insertion affordance is visible, and releasing commits ONE
 * `window.stack.group{insert_index}` — anywhere else cancels.
 */
export interface DeckDropTarget {
  readonly frameId: string;
  readonly targetWindowId: string;
  readonly insertIndex: number;
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
  readonly paletteIntent: WindowPaletteIntent | null;
  /**
   * The palette's nested-view path (ADR-003). Empty is the root palette, so
   * "reopening starts at root" is a reset rather than a rule to remember.
   * It lives beside `paletteIntent` because ⌘E raises the Sessions view from
   * outside the palette tree, exactly like the new-tab picker raises a
   * destination.
   */
  readonly paletteViewStack: readonly PaletteViewFrame[];
  readonly deckDropTarget: DeckDropTarget | null;
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
  paletteIntentRequested: { intent: WindowPaletteIntent };
  paletteIntentCleared: {};
  paletteViewPushed: { viewId: PaletteViewId };
  paletteViewPopped: {};
  paletteViewStackSet: { stack: readonly PaletteViewFrame[] };
  deckDropTargeted: { target: DeckDropTarget };
  deckDropCleared: { frameId?: string };
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
