import type { FinishLayoutGestureInput, GestureDecision } from "../lib/layout-gesture-session";
import type { SnapCorner, SnapSide } from "../lib/snap-targets";
import type { PendingWindowManagerCommand } from "./window-manager-store-types";
import type { WindowManagerStoreApi } from "./window-manager-store";

/**
 * Adapts a synchronous emitted result for imperative runtime callers. Domain
 * derivation remains inside the accepting store transition.
 */
export function advanceWindowManagerPlacementCycle(
  store: WindowManagerStoreApi,
  windowId: string,
  edge: SnapSide | SnapCorner
): number {
  let resolvedStep: number | undefined;
  const subscription = store.on("placementCycleResolved", event => {
    if (event.windowId === windowId && event.edge === edge) {
      resolvedStep = event.cycleStep;
    }
  });
  try {
    store.trigger.placementCycleAdvanced({ windowId, edge });
  } finally {
    subscription.unsubscribe();
  }
  if (resolvedStep === undefined) {
    throw new Error("Window-manager placement transition did not resolve a cycle step.");
  }
  return resolvedStep;
}

export function finishWindowManagerGesture(
  store: WindowManagerStoreApi,
  input: FinishLayoutGestureInput
): GestureDecision | null {
  let decision: GestureDecision | null = null;
  const subscription = store.on("gestureDecisionResolved", event => {
    decision = event.decision;
  });
  try {
    store.trigger.gestureFinished(input);
  } finally {
    subscription.unsubscribe();
  }
  return decision;
}

export function beginWindowManagerCommand(
  store: WindowManagerStoreApi,
  command: PendingWindowManagerCommand
): boolean {
  let accepted = false;
  const subscription = store.on("commandAccepted", event => {
    if (event.commandId === command.id) accepted = true;
  });
  try {
    store.trigger.commandBegan({ command });
  } finally {
    subscription.unsubscribe();
  }
  return accepted;
}
