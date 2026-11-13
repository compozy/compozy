import { createStore } from "@xstate/store";

import type { PaletteViewId } from "../lib/palette-view-registry";
import {
  PALETTE_VIEW_STACK_ROOT,
  popPaletteViewFrame,
  pushPaletteViewFrame,
  type PaletteViewFrame,
} from "../lib/palette-view-stack";
import type { SeamPreview } from "../lib/seam-preview";
import type { WindowManagerConnectionStatus } from "../lib/window-manager-types";
import type {
  DesktopOverviewDesktopAction,
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
  WindowPaletteIntent,
  WindowRouteIntent,
} from "./window-manager-store-types";

export type { WindowManagerConnectionStatus } from "../lib/window-manager-types";

function copyBinding(binding: WindowManagerBinding): WindowManagerBinding {
  return { ...binding };
}

function sameBinding(left: WindowManagerBinding | null, right: WindowManagerBinding): boolean {
  return (
    left?.workspaceId === right.workspaceId &&
    left.profileId === right.profileId &&
    left.clientId === right.clientId
  );
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
    left.viewportWidth === right.viewportWidth &&
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
    connectionStatus: "idle",
    activeOverlay: null,
    overviewSegmentRequest: null,
    overviewDesktopAction: null,
    transitionIntent: null,
    routeIntents: {},
    paletteIntent: null,
    paletteViewStack: PALETTE_VIEW_STACK_ROOT,
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
          viewportWidth: event.workArea.viewportWidth,
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
        overviewDesktopAction: null,
      }),
      overlayClosed: state =>
        state.activeOverlay === null &&
        state.overviewSegmentRequest === null &&
        state.overviewDesktopAction === null
          ? undefined
          : {
              ...state,
              activeOverlay: null,
              overviewSegmentRequest: null,
              overviewDesktopAction: null,
            },
      overviewSegmentRequested: (state, event: { request: DesktopOverviewSegmentRequest }) => ({
        ...state,
        activeOverlay: { kind: "desktops-overview" },
        overviewSegmentRequest: {
          ...event.request,
          hiddenDesktopIds: [...event.request.hiddenDesktopIds],
        },
        overviewDesktopAction: null,
      }),
      // The indicator menu's Rename and Close desktop finish on the overview card (US-015.EC-2).
      overviewDesktopActionRequested: (
        state,
        event: { request: DesktopOverviewDesktopAction }
      ) => ({
        ...state,
        activeOverlay: { kind: "desktops-overview" },
        overviewSegmentRequest: null,
        overviewDesktopAction: { ...event.request },
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
      // A destination scope and a view path are two answers to "what is this
      // palette showing"; letting both stand would leave the surface deciding
      // between them on every render, so the newer request wins outright.
      paletteIntentRequested: (state, event: { intent: WindowPaletteIntent }) => ({
        ...state,
        paletteIntent: { ...event.intent },
        paletteViewStack: PALETTE_VIEW_STACK_ROOT,
      }),
      paletteIntentCleared: state =>
        state.paletteIntent === null ? undefined : { ...state, paletteIntent: null },
      paletteViewPushed: (state, event: { viewId: PaletteViewId }) => ({
        ...state,
        paletteIntent: null,
        paletteViewStack: pushPaletteViewFrame(state.paletteViewStack, event.viewId),
      }),
      paletteViewPopped: state =>
        state.paletteViewStack.length === 0
          ? undefined
          : { ...state, paletteViewStack: popPaletteViewFrame(state.paletteViewStack) },
      paletteViewStackSet: (state, event: { stack: readonly PaletteViewFrame[] }) =>
        state.paletteViewStack.length === 0 && event.stack.length === 0
          ? undefined
          : {
              ...state,
              paletteIntent: event.stack.length === 0 ? state.paletteIntent : null,
              paletteViewStack: [...event.stack],
            },
      seamPreviewSet: (state, event: { preview: SeamPreview }) => {
        const preview = event.preview;
        const current = state.seamPreview;
        const unchanged =
          current !== null &&
          current.deltaPx === preview.deltaPx &&
          current.splitId === preview.splitId &&
          current.boundaryIndex === preview.boundaryIndex;
        if (unchanged) return;
        return { ...state, seamPreview: { ...preview } };
      },
      seamPreviewCleared: state =>
        state.seamPreview === null ? undefined : { ...state, seamPreview: null },
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
  DesktopOverviewDesktopAction,
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
  WindowPaletteIntent,
  WindowRouteIntent,
} from "./window-manager-store-types";

export {
  selectDesktopOverviewDesktopAction,
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
  selectWindowManagerBinding,
  selectWindowManagerConflict,
  selectWindowManagerConnectionStatus,
  selectWindowManagerDiagnostic,
  selectWindowManagerOverlay,
  selectWindowManagerWorkArea,
} from "./window-manager-store-selectors";
