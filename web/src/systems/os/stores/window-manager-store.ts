import { createStore } from "@xstate/store";

import {
  beginLayoutGesture,
  cancelLayoutGesture,
  finishLayoutGesture,
  previewLayoutGesture,
} from "../lib/layout-gesture-session";
import type { SeamPreview } from "../lib/seam-preview";
import type { SnapCorner, SnapSide } from "../lib/snap-targets";
import type { WindowManagerConnectionStatus } from "../lib/window-manager-types";
import type {
  DesktopOverviewSegmentRequest,
  DesktopTransitionIntent,
  PendingWindowManagerCommand,
  WindowManagerBinding,
  WindowManagerCommandState,
  WindowManagerDiagnostic,
  WindowManagerOverlay,
  WindowManagerRevisionConflict,
  WindowManagerStoreEmitted,
  WindowManagerStoreEvents,
  WindowManagerStoreState,
  WindowManagerWorkArea,
  WindowRouteIntent,
} from "./window-manager-store-types";

export type { WindowManagerConnectionStatus } from "../lib/window-manager-types";

function copyBinding(binding: WindowManagerBinding): WindowManagerBinding {
  return { ...binding };
}

function sameBinding(left: WindowManagerBinding | null, right: WindowManagerBinding): boolean {
  return left?.workspaceId === right.workspaceId && left.clientId === right.clientId;
}

function fencedBinding(state: WindowManagerStoreState, binding?: WindowManagerBinding): boolean {
  return binding === undefined || sameBinding(state.binding, binding);
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

function idleCommandState(
  diagnostic: WindowManagerDiagnostic | null = null
): WindowManagerCommandState {
  return { status: "idle", diagnostic };
}

function bindingScopedState(
  binding: WindowManagerBinding | null
): Omit<WindowManagerStoreState, "workArea"> {
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

function initialWindowManagerState(): WindowManagerStoreState {
  return { ...bindingScopedState(null), workArea: null };
}

export function createWindowManagerStore() {
  return createStore<WindowManagerStoreState, WindowManagerStoreEvents, WindowManagerStoreEmitted>({
    context: initialWindowManagerState(),
    on: {
      bindingBound: (state, event: { binding: WindowManagerBinding }) => {
        if (sameBinding(state.binding, event.binding)) return;
        return { ...bindingScopedState(copyBinding(event.binding)), workArea: state.workArea };
      },
      bindingUnbound: state => ({ ...bindingScopedState(null), workArea: state.workArea }),
      connectionStatusChanged: (state, event: { status: WindowManagerConnectionStatus }) =>
        state.connectionStatus === event.status
          ? undefined
          : { ...state, connectionStatus: event.status },
      workAreaMeasured: (state, event: { workArea: WindowManagerWorkArea | null }) => {
        if (sameWorkArea(state.workArea, event.workArea)) return;
        const workArea = event.workArea && {
          rect: { ...event.workArea.rect },
          origin: { ...event.workArea.origin },
        };
        return { ...state, workArea };
      },
      overlayOpened: (state, event: { overlay: WindowManagerOverlay }) => ({
        ...state,
        activeOverlay:
          event.overlay.kind === "layout-editor"
            ? { ...event.overlay }
            : { kind: "desktops-overview" },
        overviewSegmentRequest: null,
      }),
      overlayClosed: state =>
        state.activeOverlay === null && state.overviewSegmentRequest === null
          ? undefined
          : { ...state, activeOverlay: null, overviewSegmentRequest: null },
      overviewSegmentRequested: (state, event: { request: DesktopOverviewSegmentRequest }) => ({
        ...state,
        activeOverlay: { kind: "desktops-overview" },
        overviewSegmentRequest: {
          ...event.request,
          hiddenDesktopIds: [...event.request.hiddenDesktopIds],
        },
      }),
      desktopStateObserved: (
        state,
        event: { activeDesktopId: string; reconciledIntent: DesktopTransitionIntent | null }
      ) => {
        const current = state.transitionIntent;
        if (current?.mode === "instant" && current.toDesktopId === event.activeDesktopId) {
          return { ...state, transitionIntent: null };
        }
        if (current !== null || event.reconciledIntent === null) return;
        return { ...state, transitionIntent: { ...event.reconciledIntent } };
      },
      transitionIntentChanged: (state, event: { intent: DesktopTransitionIntent | null }) => ({
        ...state,
        transitionIntent: event.intent === null ? null : { ...event.intent },
      }),
      transitionIntentRejected: (
        state,
        event: { binding: WindowManagerBinding; toDesktopId: string }
      ) =>
        fencedBinding(state, event.binding) &&
        state.transitionIntent?.toDesktopId === event.toDesktopId
          ? { ...state, transitionIntent: null }
          : undefined,
      routeIntentSet: (state, event: { intent: WindowRouteIntent }) => ({
        ...state,
        routeIntents: {
          ...state.routeIntents,
          [event.intent.windowId]: {
            ...event.intent,
            route: {
              pathname: event.intent.route.pathname,
              search: { ...event.intent.route.search },
            },
          },
        },
      }),
      routeIntentCleared: (state, event: { windowId: string; intentId: string }) => {
        const current = state.routeIntents[event.windowId];
        if (current?.id !== event.intentId) return;
        const routeIntents = { ...state.routeIntents };
        delete routeIntents[event.windowId];
        return { ...state, routeIntents };
      },
      placementCycleAdvanced: (state, event, enqueue) => {
        const current = state.placementCycles[event.windowId];
        const cycleStep = current?.edge === event.edge ? current.nextStep : 0;
        enqueue.emit.placementCycleResolved({
          windowId: event.windowId,
          edge: event.edge,
          cycleStep,
        });
        return {
          ...state,
          placementCycles: {
            ...state.placementCycles,
            [event.windowId]: { edge: event.edge, nextStep: cycleStep + 1 },
          },
        };
      },
      placementTargetTracked: (
        state,
        event: { windowId: string; edge: SnapSide | SnapCorner | null }
      ) => {
        const current = state.placementCycles[event.windowId];
        if (event.edge === null) {
          if (current === undefined) return;
          const placementCycles = { ...state.placementCycles };
          delete placementCycles[event.windowId];
          return { ...state, placementCycles };
        }
        if (current?.edge === event.edge) return;
        return {
          ...state,
          placementCycles: {
            ...state.placementCycles,
            [event.windowId]: { edge: event.edge, nextStep: 1 },
          },
        };
      },
      seamPreviewSet: (state, event: { preview: SeamPreview }) => {
        const preview = event.preview;
        if (
          state.seamPreview?.splitId === preview.splitId &&
          state.seamPreview.boundaryIndex === preview.boundaryIndex &&
          state.seamPreview.deltaPx === preview.deltaPx
        ) {
          return;
        }
        return { ...state, seamPreview: { ...preview } };
      },
      seamPreviewCleared: state =>
        state.seamPreview === null ? undefined : { ...state, seamPreview: null },
      gestureBegan: (state, event) => ({ ...state, gesture: beginLayoutGesture(event) }),
      gesturePreviewed: (state, event) => {
        if (state.gesture === null) return;
        const gesture = previewLayoutGesture(
          state.gesture,
          event.point,
          event.preview,
          event.currentWorkArea
        );
        return gesture === state.gesture ? undefined : { ...state, gesture };
      },
      gestureCancelled: (state, event) => {
        if (state.gesture === null) return;
        const gesture = cancelLayoutGesture(state.gesture, event.reason, event.point);
        return gesture === state.gesture ? undefined : { ...state, gesture };
      },
      gestureFinished: (state, event, enqueue) => {
        if (state.gesture === null) return;
        const decision = finishLayoutGesture(state.gesture, event);
        const gesture =
          decision.kind === "cancel"
            ? cancelLayoutGesture(state.gesture, decision.reason, decision.finalPoint)
            : null;
        enqueue.emit.gestureDecisionResolved({ decision });
        return { ...state, gesture };
      },
      gestureCleared: state => (state.gesture === null ? undefined : { ...state, gesture: null }),
      commandBegan: (state, event: { command: PendingWindowManagerCommand }, enqueue) => {
        if (state.commandState.status !== "idle") return;
        enqueue.emit.commandAccepted({ commandId: event.command.id });
        return {
          ...state,
          commandState: { status: "pending", command: { ...event.command }, diagnostic: null },
        };
      },
      commandCompleted: (
        state,
        event: {
          commandId: string;
          diagnostic?: WindowManagerDiagnostic;
          binding?: WindowManagerBinding;
        }
      ) =>
        state.commandState.status !== "pending" ||
        state.commandState.command.id !== event.commandId ||
        !fencedBinding(state, event.binding)
          ? undefined
          : {
              ...state,
              commandState: idleCommandState(event.diagnostic ? { ...event.diagnostic } : null),
            },
      commandFailed: (
        state,
        event: {
          commandId: string;
          diagnostic: WindowManagerDiagnostic;
          binding?: WindowManagerBinding;
        }
      ) =>
        state.commandState.status !== "pending" ||
        state.commandState.command.id !== event.commandId ||
        !fencedBinding(state, event.binding)
          ? undefined
          : { ...state, commandState: idleCommandState({ ...event.diagnostic }) },
      conflictRecorded: (
        state,
        event: {
          conflict: WindowManagerRevisionConflict;
          diagnostic: WindowManagerDiagnostic;
          binding?: WindowManagerBinding;
        }
      ) =>
        state.commandState.status !== "pending" ||
        state.commandState.command.id !== event.conflict.commandId ||
        !fencedBinding(state, event.binding)
          ? undefined
          : {
              ...state,
              commandState: {
                status: "conflict",
                conflict: { ...event.conflict },
                diagnostic: { ...event.diagnostic },
              },
            },
      conflictCleared: state =>
        state.commandState.status !== "conflict"
          ? undefined
          : { ...state, commandState: idleCommandState() },
      diagnosticReported: (state, event: { diagnostic: WindowManagerDiagnostic }) => ({
        ...state,
        commandState: { ...state.commandState, diagnostic: { ...event.diagnostic } },
      }),
      diagnosticCleared: state =>
        state.commandState.status === "conflict" || state.commandState.diagnostic === null
          ? undefined
          : { ...state, commandState: { ...state.commandState, diagnostic: null } },
    },
  });
}

export type WindowManagerStoreApi = ReturnType<typeof createWindowManagerStore>;

export const windowManagerStore = createWindowManagerStore();

export type {
  DesktopOverviewSegmentRequest,
  DesktopTransitionIntent,
  PendingWindowManagerCommand,
  WindowManagerBinding,
  WindowManagerCommandState,
  WindowManagerDiagnostic,
  WindowManagerOverlay,
  WindowManagerRevisionConflict,
  WindowManagerStoreState,
  WindowManagerWorkArea,
  WindowPlacementCycle,
  WindowRouteIntent,
} from "./window-manager-store-types";

export {
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
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
