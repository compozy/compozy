import { createStore, type StoreApi } from "zustand/vanilla";

import {
  beginLayoutGesture,
  cancelLayoutGesture,
  finishLayoutGesture,
  previewLayoutGesture,
  type BeginLayoutGestureInput,
  type FinishLayoutGestureInput,
  type GestureCancelReason,
  type GestureDecision,
  type LayoutGestureSession,
} from "../lib/layout-gesture-session";
import type { SeamPreview } from "../lib/seam-preview";
import type { SnapCorner, SnapSide, SnapTarget } from "../lib/snap-targets";
import type { OsWindowRoute } from "../lib/os-types";
import type {
  DesktopId,
  LayoutRevision,
  PixelPoint,
  PixelRect,
  WindowManagerCommandId,
  WindowManagerConnectionStatus,
} from "../lib/window-manager-types";

export type { WindowManagerConnectionStatus } from "../lib/window-manager-types";

export interface WindowManagerBinding {
  readonly workspaceId: string;
  readonly clientId: string;
}

export interface WindowManagerWorkArea {
  readonly rect: PixelRect;
  /** Viewport offset of the layer origin — converts client to layer coordinates. */
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

export interface WindowManagerActions {
  bindClient: (binding: WindowManagerBinding) => void;
  unbindClient: () => void;
  setConnectionStatus: (status: WindowManagerConnectionStatus) => void;
  setWorkArea: (workArea: WindowManagerWorkArea | null) => void;
  openOverlay: (overlay: WindowManagerOverlay) => void;
  closeOverlay: () => void;
  requestOverviewSegment: (request: DesktopOverviewSegmentRequest) => void;
  setTransitionIntent: (intent: DesktopTransitionIntent | null) => void;
  setRouteIntent: (intent: WindowRouteIntent) => void;
  clearRouteIntent: (windowId: string, intentId: string) => void;
  nextPlacementCycle: (windowId: string, edge: SnapSide | SnapCorner) => number;
  trackPlacementTarget: (windowId: string, edge: SnapSide | SnapCorner | null) => void;
  setSeamPreview: (preview: SeamPreview) => void;
  clearSeamPreview: () => void;
  beginGesture: (input: BeginLayoutGestureInput) => LayoutGestureSession;
  previewGesture: (
    point: PixelPoint,
    preview: SnapTarget | null,
    currentWorkArea: PixelRect
  ) => void;
  cancelGesture: (reason: GestureCancelReason, point?: PixelPoint) => LayoutGestureSession | null;
  finishGesture: (input: FinishLayoutGestureInput) => GestureDecision | null;
  clearGesture: () => void;
  beginCommand: (command: PendingWindowManagerCommand) => boolean;
  completeCommand: (commandId: string, diagnostic?: WindowManagerDiagnostic) => void;
  failCommand: (commandId: string, diagnostic: WindowManagerDiagnostic) => void;
  recordConflict: (
    conflict: WindowManagerRevisionConflict,
    diagnostic: WindowManagerDiagnostic
  ) => void;
  clearConflict: () => void;
  reportDiagnostic: (diagnostic: WindowManagerDiagnostic) => void;
  clearDiagnostic: () => void;
}

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
  /** Live seam-drag preview applied to the projection until commit or cancel. */
  readonly seamPreview: SeamPreview | null;
  readonly commandState: WindowManagerCommandState;
  readonly actions: WindowManagerActions;
}

export type WindowManagerStoreApi = StoreApi<WindowManagerStoreState>;

type BindingScopedWindowManagerState = Omit<WindowManagerStoreState, "actions" | "workArea">;

function copyBinding(binding: WindowManagerBinding): WindowManagerBinding {
  return { workspaceId: binding.workspaceId, clientId: binding.clientId };
}

function copyWorkArea(workArea: WindowManagerWorkArea): WindowManagerWorkArea {
  return { rect: { ...workArea.rect }, origin: { ...workArea.origin } };
}

function sameWorkArea(
  left: WindowManagerWorkArea | null,
  right: WindowManagerWorkArea | null
): boolean {
  if (left === null || right === null) return left === right;
  return (
    left.rect.x === right.rect.x &&
    left.rect.y === right.rect.y &&
    left.rect.w === right.rect.w &&
    left.rect.h === right.rect.h &&
    left.origin.x === right.origin.x &&
    left.origin.y === right.origin.y
  );
}

function copyOverlay(overlay: WindowManagerOverlay): WindowManagerOverlay {
  return overlay.kind === "layout-editor"
    ? { kind: overlay.kind, desktopId: overlay.desktopId }
    : { kind: overlay.kind };
}

function copyOverviewRequest(
  request: DesktopOverviewSegmentRequest
): DesktopOverviewSegmentRequest {
  return {
    direction: request.direction,
    hiddenDesktopIds: [...request.hiddenDesktopIds],
    anchorDesktopId: request.anchorDesktopId,
  };
}

function copyTransitionIntent(intent: DesktopTransitionIntent): DesktopTransitionIntent {
  return { ...intent };
}

function copyRouteIntent(intent: WindowRouteIntent): WindowRouteIntent {
  return {
    id: intent.id,
    windowId: intent.windowId,
    route: { pathname: intent.route.pathname, search: { ...intent.route.search } },
  };
}

function copyCommand(command: PendingWindowManagerCommand): PendingWindowManagerCommand {
  return { ...command };
}

function copyConflict(conflict: WindowManagerRevisionConflict): WindowManagerRevisionConflict {
  return { ...conflict };
}

function copyDiagnostic(diagnostic: WindowManagerDiagnostic): WindowManagerDiagnostic {
  return { ...diagnostic };
}

function idleCommandState(
  diagnostic: WindowManagerDiagnostic | null = null
): WindowManagerCommandState {
  return { status: "idle", diagnostic };
}

function bindingScopedState(binding: WindowManagerBinding | null): BindingScopedWindowManagerState {
  return {
    binding,
    connectionStatus: "disconnected",
    activeOverlay: null,
    overviewSegmentRequest: null,
    transitionIntent: null,
    routeIntents: {},
    placementCycles: {},
    gesture: null,
    seamPreview: null,
    commandState: idleCommandState(),
  };
}

function sameBinding(left: WindowManagerBinding | null, right: WindowManagerBinding): boolean {
  return left?.workspaceId === right.workspaceId && left.clientId === right.clientId;
}

export function createWindowManagerStore(): WindowManagerStoreApi {
  return createStore<WindowManagerStoreState>()((set, get) => {
    const actions: WindowManagerActions = {
      bindClient: binding => {
        if (sameBinding(get().binding, binding)) return;
        set(bindingScopedState(copyBinding(binding)));
      },

      unbindClient: () => {
        set(bindingScopedState(null));
      },

      setConnectionStatus: connectionStatus => {
        if (get().connectionStatus === connectionStatus) return;
        set({ connectionStatus });
      },

      setWorkArea: workArea => {
        if (sameWorkArea(get().workArea, workArea)) return;
        set({ workArea: workArea === null ? null : copyWorkArea(workArea) });
      },

      openOverlay: activeOverlay => {
        set({ activeOverlay: copyOverlay(activeOverlay), overviewSegmentRequest: null });
      },

      closeOverlay: () => {
        const state = get();
        if (state.activeOverlay === null && state.overviewSegmentRequest === null) return;
        set({ activeOverlay: null, overviewSegmentRequest: null });
      },

      requestOverviewSegment: request => {
        set({
          activeOverlay: { kind: "desktops-overview" },
          overviewSegmentRequest: copyOverviewRequest(request),
        });
      },

      setTransitionIntent: transitionIntent => {
        set({
          transitionIntent:
            transitionIntent === null ? null : copyTransitionIntent(transitionIntent),
        });
      },

      setRouteIntent: intent => {
        set({
          routeIntents: {
            ...get().routeIntents,
            [intent.windowId]: copyRouteIntent(intent),
          },
        });
      },

      clearRouteIntent: (windowId, intentId) => {
        const current = get().routeIntents[windowId];
        if (current?.id !== intentId) return;
        const routeIntents = { ...get().routeIntents };
        delete routeIntents[windowId];
        set({ routeIntents });
      },

      nextPlacementCycle: (windowId, edge) => {
        const current = get().placementCycles[windowId];
        const cycleStep = current?.edge === edge ? current.nextStep : 0;
        set({
          placementCycles: {
            ...get().placementCycles,
            [windowId]: { edge, nextStep: cycleStep + 1 },
          },
        });
        return cycleStep;
      },

      trackPlacementTarget: (windowId, edge) => {
        const placementCycles = get().placementCycles;
        const current = placementCycles[windowId];
        if (edge === null) {
          if (current === undefined) return;
          const remaining = Object.fromEntries(
            Object.entries(placementCycles).filter(([id]) => id !== windowId)
          );
          set({ placementCycles: remaining });
          return;
        }
        if (current?.edge === edge) return;
        set({
          placementCycles: {
            ...placementCycles,
            [windowId]: { edge, nextStep: 1 },
          },
        });
      },

      setSeamPreview: preview => {
        const current = get().seamPreview;
        if (
          current !== null &&
          current.splitId === preview.splitId &&
          current.boundaryIndex === preview.boundaryIndex &&
          current.deltaPx === preview.deltaPx
        ) {
          return;
        }
        set({
          seamPreview: {
            splitId: preview.splitId,
            boundaryIndex: preview.boundaryIndex,
            deltaPx: preview.deltaPx,
          },
        });
      },

      clearSeamPreview: () => {
        if (get().seamPreview === null) return;
        set({ seamPreview: null });
      },

      beginGesture: input => {
        const gesture = beginLayoutGesture(input);
        set({ gesture });
        return gesture;
      },

      previewGesture: (point, preview, currentWorkArea) => {
        const current = get().gesture;
        if (current === null) return;
        const gesture = previewLayoutGesture(current, point, preview, currentWorkArea);
        if (gesture !== current) set({ gesture });
      },

      cancelGesture: (reason, point) => {
        const current = get().gesture;
        if (current === null) return null;
        const gesture = cancelLayoutGesture(current, reason, point);
        if (gesture !== current) set({ gesture });
        return gesture;
      },

      finishGesture: input => {
        const current = get().gesture;
        if (current === null) return null;
        const decision = finishLayoutGesture(current, input);
        set({
          gesture:
            decision.kind === "cancel"
              ? cancelLayoutGesture(current, decision.reason, decision.finalPoint)
              : null,
        });
        return decision;
      },

      clearGesture: () => {
        if (get().gesture === null) return;
        set({ gesture: null });
      },

      beginCommand: command => {
        if (get().commandState.status !== "idle") return false;
        set({
          commandState: {
            status: "pending",
            command: copyCommand(command),
            diagnostic: null,
          },
        });
        return true;
      },

      completeCommand: (commandId, diagnostic) => {
        const current = get().commandState;
        if (current.status !== "pending" || current.command.id !== commandId) return;
        set({
          commandState: idleCommandState(
            diagnostic === undefined ? null : copyDiagnostic(diagnostic)
          ),
        });
      },

      failCommand: (commandId, diagnostic) => {
        const current = get().commandState;
        if (current.status !== "pending" || current.command.id !== commandId) return;
        set({ commandState: idleCommandState(copyDiagnostic(diagnostic)) });
      },

      recordConflict: (conflict, diagnostic) => {
        const current = get().commandState;
        if (current.status !== "pending" || current.command.id !== conflict.commandId) return;
        set({
          commandState: {
            status: "conflict",
            conflict: copyConflict(conflict),
            diagnostic: copyDiagnostic(diagnostic),
          },
        });
      },

      clearConflict: () => {
        if (get().commandState.status !== "conflict") return;
        set({ commandState: idleCommandState() });
      },

      reportDiagnostic: diagnostic => {
        const current = get().commandState;
        set({ commandState: { ...current, diagnostic: copyDiagnostic(diagnostic) } });
      },

      clearDiagnostic: () => {
        const current = get().commandState;
        if (current.status === "conflict" || current.diagnostic === null) return;
        set({ commandState: { ...current, diagnostic: null } });
      },
    };

    return { ...bindingScopedState(null), workArea: null, actions };
  });
}

export const windowManagerStore = createWindowManagerStore();

export {
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
  selectWindowManagerActions,
  selectWindowManagerBinding,
  selectWindowManagerConflict,
  selectWindowManagerConnectionStatus,
  selectWindowManagerDiagnostic,
  selectWindowManagerGesture,
  selectWindowManagerGestureActive,
  selectWindowManagerGesturePreview,
  selectWindowManagerOverlay,
  selectWindowManagerWorkArea,
} from "./window-manager-store-selectors";
