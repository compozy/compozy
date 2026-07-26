// Suite: window-manager shared browser state
// Invariant: WM-WEB-007 — Query remains the only snapshot authority; surface geometry survives
// binding changes while client interaction state resets per binding.
// Boundary IN: the Zustand store, its grouped actions, focused React selectors, and pure gesture delegation.
// Boundary OUT: TanStack Query snapshots/events, daemon commands, Rnd pointer wiring, and durable persistence.
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useWindowManagerGestureActive } from "../../hooks/use-window-manager-store";
import type { SnapTarget } from "../../lib/snap-targets";
import {
  createWindowManagerStore,
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
  selectWindowManagerActions,
  selectWindowManagerBinding,
  selectWindowManagerConflict,
  selectWindowManagerConnectionStatus,
  selectWindowManagerDiagnostic,
  selectWindowManagerGesture,
  selectWindowManagerGesturePreview,
  selectWindowManagerOverlay,
  selectWindowManagerWorkArea,
  type WindowManagerDiagnostic,
  type WindowManagerStoreApi,
  windowManagerStore,
} from "../window-manager-store";

const SNAP_TARGET: SnapTarget = {
  kind: "tile",
  id: "edge:left:0.5",
  edge: "left",
  ratio: 0.5,
  rect: { x: 0, y: 0, w: 600, h: 800 },
};

const GESTURE_WORK_AREA = { x: 0, y: 0, w: 1200, h: 800 } as const;

const CONFLICT_DIAGNOSTIC: WindowManagerDiagnostic = {
  code: "layout_revision_conflict",
  message: "The layout changed before the command completed.",
  severity: "warning",
  field: "expected_revision",
};

function beginGesture(store: WindowManagerStoreApi, layoutRevision = 17) {
  return store.getState().actions.beginGesture({
    pointerId: 4,
    point: { x: 420, y: 180 },
    workArea: GESTURE_WORK_AREA,
    layoutRevision,
    source: {
      windowId: "window:tasks",
      nodeId: "node:tasks",
      groupId: "group:primary",
      moveMode: "window",
    },
  });
}

describe("window manager store", () => {
  it("Should keep daemon snapshot shapes out of the client interaction authority", () => {
    const store = createWindowManagerStore();
    const state = store.getState();

    expect(state).not.toHaveProperty("snapshot");
    expect(state).not.toHaveProperty("revision");
    expect(state).not.toHaveProperty("desktops");
    expect(state).not.toHaveProperty("windows");
    expect(selectWindowManagerBinding(state)).toBeNull();
    expect(selectWindowManagerConnectionStatus(state)).toBe("disconnected");
    expect(selectWindowManagerActions(state)).toBe(state.actions);
  });

  it("Should preserve surface geometry while resetting binding-scoped state", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    actions.bindClient({ workspaceId: "workspace:alpha", clientId: "client:one" });
    actions.setConnectionStatus("connected");
    actions.setWorkArea({ rect: { x: 0, y: 0, w: 1440, h: 820 }, origin: { x: 0, y: 0 } });
    actions.requestOverviewSegment({
      direction: "later",
      hiddenDesktopIds: ["desktop:three", "desktop:four"],
      anchorDesktopId: "desktop:two",
    });
    actions.setTransitionIntent({
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:two",
      direction: "later",
      mode: "slide",
    });
    beginGesture(store);
    actions.previewGesture({ x: 2, y: 300 }, SNAP_TARGET, GESTURE_WORK_AREA);
    expect(
      actions.beginCommand({ id: "command:one", kind: "window.move", expectedRevision: 17 })
    ).toBe(true);
    actions.recordConflict(
      { commandId: "command:one", expectedRevision: 17, currentRevision: 18 },
      CONFLICT_DIAGNOSTIC
    );

    actions.bindClient({ workspaceId: "workspace:beta", clientId: "client:two" });

    const state = store.getState();
    expect(selectWindowManagerBinding(state)).toEqual({
      workspaceId: "workspace:beta",
      clientId: "client:two",
    });
    expect(selectWindowManagerConnectionStatus(state)).toBe("disconnected");
    expect(selectWindowManagerWorkArea(state)).toEqual({
      rect: { x: 0, y: 0, w: 1440, h: 820 },
      origin: { x: 0, y: 0 },
    });
    expect(selectWindowManagerOverlay(state)).toBeNull();
    expect(selectDesktopOverviewSegmentRequest(state)).toBeNull();
    expect(selectDesktopTransitionIntent(state)).toBeNull();
    expect(state.placementCycles).toEqual({});
    expect(selectWindowManagerGesture(state)).toBeNull();
    expect(selectPendingWindowManagerCommand(state)).toBeNull();
    expect(selectWindowManagerConflict(state)).toBeNull();
    expect(selectWindowManagerDiagnostic(state)).toBeNull();
    expect(state.actions).toBe(actions);

    actions.setConnectionStatus("connected");
    actions.openOverlay({ kind: "layout-editor", desktopId: "desktop:two" });
    actions.nextPlacementCycle("window:tasks", "left");
    beginGesture(store, 18);
    expect(
      actions.beginCommand({ id: "command:two", kind: "window.move", expectedRevision: 18 })
    ).toBe(true);

    actions.unbindClient();

    const unbound = store.getState();
    expect(selectWindowManagerBinding(unbound)).toBeNull();
    expect(selectWindowManagerConnectionStatus(unbound)).toBe("disconnected");
    expect(selectWindowManagerWorkArea(unbound)).toEqual({
      rect: { x: 0, y: 0, w: 1440, h: 820 },
      origin: { x: 0, y: 0 },
    });
    expect(selectWindowManagerOverlay(unbound)).toBeNull();
    expect(unbound.placementCycles).toEqual({});
    expect(selectWindowManagerGesture(unbound)).toBeNull();
    expect(selectPendingWindowManagerCommand(unbound)).toBeNull();
  });

  it("Should preserve transient state when the identical binding is announced again", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    const binding = { workspaceId: "workspace:alpha", clientId: "client:one" };
    actions.bindClient(binding);
    actions.setConnectionStatus("connected");
    actions.openOverlay({ kind: "layout-editor", desktopId: "desktop:one" });
    const before = store.getState();

    actions.bindClient(binding);

    expect(store.getState()).toBe(before);
    expect(selectWindowManagerConnectionStatus(store.getState())).toBe("connected");
    expect(selectWindowManagerOverlay(store.getState())).toEqual({
      kind: "layout-editor",
      desktopId: "desktop:one",
    });
  });

  it("Should keep one exclusive overlay and scope an overflow request to the overview", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    const hiddenDesktopIds = ["desktop:one", "desktop:two"];
    actions.openOverlay({ kind: "layout-editor", desktopId: "desktop:build" });

    actions.requestOverviewSegment({
      direction: "earlier",
      hiddenDesktopIds,
      anchorDesktopId: "desktop:five",
    });
    hiddenDesktopIds.push("desktop:external-mutation");

    let state = store.getState();
    expect(selectWindowManagerOverlay(state)).toEqual({ kind: "desktops-overview" });
    expect(selectDesktopOverviewSegmentRequest(state)).toEqual({
      direction: "earlier",
      hiddenDesktopIds: ["desktop:one", "desktop:two"],
      anchorDesktopId: "desktop:five",
    });

    actions.closeOverlay();
    state = store.getState();
    expect(selectWindowManagerOverlay(state)).toBeNull();
    expect(selectDesktopOverviewSegmentRequest(state)).toBeNull();

    actions.requestOverviewSegment({
      direction: "later",
      hiddenDesktopIds: ["desktop:six"],
      anchorDesktopId: "desktop:five",
    });
    actions.openOverlay({ kind: "desktops-overview" });
    state = store.getState();
    expect(selectWindowManagerOverlay(state)).toEqual({ kind: "desktops-overview" });
    expect(selectDesktopOverviewSegmentRequest(state)).toBeNull();

    actions.openOverlay({ kind: "layout-editor", desktopId: "desktop:build" });
    expect(selectWindowManagerOverlay(store.getState())).toEqual({
      kind: "layout-editor",
      desktopId: "desktop:build",
    });
  });

  it("Should isolate measured work-area and transition inputs from caller mutation", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    let notifications = 0;
    const unsubscribe = store.subscribe(() => {
      notifications += 1;
    });
    const rect = { x: 10, y: 20, w: 1200, h: 760 };
    const transition = {
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:two",
      direction: "later" as const,
      mode: "slide" as const,
    };

    actions.setWorkArea({ rect, origin: { x: 0, y: 44 } });
    const ownedWorkArea = selectWindowManagerWorkArea(store.getState());
    notifications = 0;
    actions.setWorkArea({ rect: { x: 10, y: 20, w: 1200, h: 760 }, origin: { x: 0, y: 44 } });
    expect(selectWindowManagerWorkArea(store.getState())).toBe(ownedWorkArea);
    expect(notifications).toBe(0);
    actions.setTransitionIntent(transition);
    rect.x = 900;
    transition.toDesktopId = "desktop:external-mutation";

    expect(selectWindowManagerWorkArea(store.getState())).toEqual({
      rect: { x: 10, y: 20, w: 1200, h: 760 },
      origin: { x: 0, y: 44 },
    });
    expect(selectDesktopTransitionIntent(store.getState())).toEqual({
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:two",
      direction: "later",
      mode: "slide",
    });
    unsubscribe();
  });

  it("Should cycle placements per window and reset on side or structural target changes", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;

    expect(actions.nextPlacementCycle("window:tasks", "left")).toBe(0);
    expect(actions.nextPlacementCycle("window:tasks", "left")).toBe(1);
    expect(actions.nextPlacementCycle("window:agents", "left")).toBe(0);
    expect(actions.nextPlacementCycle("window:tasks", "right")).toBe(0);

    actions.trackPlacementTarget("window:tasks", "top");
    expect(actions.nextPlacementCycle("window:tasks", "top")).toBe(1);

    actions.trackPlacementTarget("window:tasks", null);
    expect(actions.nextPlacementCycle("window:tasks", "top")).toBe(0);
  });

  it("Should capture a gesture revision and clear its preview on explicit cancellation", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    beginGesture(store, 23);

    const previewTarget: SnapTarget = { ...SNAP_TARGET, rect: { ...SNAP_TARGET.rect } };
    actions.previewGesture({ x: 1, y: 240 }, previewTarget, GESTURE_WORK_AREA);
    previewTarget.rect.x = 500;

    expect(selectWindowManagerGesturePreview(store.getState())).toEqual(SNAP_TARGET);
    expect(selectWindowManagerGesture(store.getState())).toMatchObject({
      status: "active",
      layoutRevision: 23,
      currentPoint: { x: 1, y: 240 },
    });

    actions.cancelGesture("escape", { x: 4, y: 245 });

    expect(selectWindowManagerGesturePreview(store.getState())).toBeNull();
    expect(selectWindowManagerGesture(store.getState())).toMatchObject({
      status: "cancelled",
      reason: "escape",
      finalPoint: { x: 4, y: 245 },
      layoutRevision: 23,
    });
  });

  it("Should notify only the dragged window when gesture activity changes, not on previews", () => {
    const actions = windowManagerStore.getState().actions;
    actions.unbindClient();
    let draggedWindowRenders = 0;
    let idleWindowRenders = 0;
    const draggedWindow = renderHook(() => {
      draggedWindowRenders += 1;
      return useWindowManagerGestureActive("window:tasks");
    });
    const idleWindow = renderHook(() => {
      idleWindowRenders += 1;
      return useWindowManagerGestureActive("window:vault");
    });
    const initialDraggedWindowRenders = draggedWindowRenders;
    const initialIdleWindowRenders = idleWindowRenders;

    act(() => {
      beginGesture(windowManagerStore);
    });

    expect(draggedWindow.result.current).toBe(true);
    expect(draggedWindowRenders).toBeGreaterThan(initialDraggedWindowRenders);
    expect(idleWindow.result.current).toBe(false);
    expect(idleWindowRenders).toBe(initialIdleWindowRenders);
    const activeDraggedWindowRenders = draggedWindowRenders;

    act(() => {
      actions.previewGesture({ x: 12, y: 240 }, SNAP_TARGET, GESTURE_WORK_AREA);
      actions.previewGesture({ x: 24, y: 260 }, SNAP_TARGET, GESTURE_WORK_AREA);
    });

    expect(draggedWindow.result.current).toBe(true);
    expect(draggedWindowRenders).toBe(activeDraggedWindowRenders);
    expect(idleWindowRenders).toBe(initialIdleWindowRenders);

    act(() => {
      actions.cancelGesture("escape");
    });

    expect(draggedWindow.result.current).toBe(false);
    expect(draggedWindowRenders).toBeGreaterThan(activeDraggedWindowRenders);
    expect(idleWindowRenders).toBe(initialIdleWindowRenders);
  });

  it("Should finish through the pure gesture decision without persisting the session", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    beginGesture(store, 31);
    actions.previewGesture({ x: 2, y: 300 }, SNAP_TARGET, GESTURE_WORK_AREA);

    const decision = actions.finishGesture({
      finalPoint: { x: 1, y: 301 },
      finalTarget: SNAP_TARGET,
      currentRevision: 31,
      currentWorkArea: GESTURE_WORK_AREA,
      clientValid: true,
    });

    expect(decision).toMatchObject({
      kind: "commit",
      command: { expectedRevision: 31, finalPoint: { x: 1, y: 301 } },
    });
    expect(selectWindowManagerGesture(store.getState())).toBeNull();
    expect(selectWindowManagerGesture(createWindowManagerStore().getState())).toBeNull();
  });

  it("Should serialize pending commands and record conflicts only for the active command", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    expect(
      actions.beginCommand({ id: "command:one", kind: "layout.arrange", expectedRevision: 40 })
    ).toBe(true);
    expect(
      actions.beginCommand({ id: "command:two", kind: "layout.balance", expectedRevision: 40 })
    ).toBe(false);
    expect(selectPendingWindowManagerCommand(store.getState())).toEqual({
      id: "command:one",
      kind: "layout.arrange",
      expectedRevision: 40,
    });

    actions.recordConflict(
      { commandId: "command:stale", expectedRevision: 39, currentRevision: 41 },
      CONFLICT_DIAGNOSTIC
    );
    expect(selectPendingWindowManagerCommand(store.getState())?.id).toBe("command:one");

    actions.recordConflict(
      { commandId: "command:one", expectedRevision: 40, currentRevision: 41 },
      CONFLICT_DIAGNOSTIC
    );
    expect(selectPendingWindowManagerCommand(store.getState())).toBeNull();
    expect(selectWindowManagerConflict(store.getState())).toEqual({
      commandId: "command:one",
      expectedRevision: 40,
      currentRevision: 41,
    });
    expect(selectWindowManagerDiagnostic(store.getState())).toEqual(CONFLICT_DIAGNOSTIC);

    expect(
      actions.beginCommand({ id: "command:two", kind: "layout.balance", expectedRevision: 41 })
    ).toBe(false);
    actions.clearConflict();
    expect(selectWindowManagerConflict(store.getState())).toBeNull();
    expect(selectWindowManagerDiagnostic(store.getState())).toBeNull();
    expect(store.getState().actions).toBe(actions);
  });

  it("Should ignore stale completions and expose a terminal command diagnostic", () => {
    const store = createWindowManagerStore();
    const actions = store.getState().actions;
    expect(
      actions.beginCommand({ id: "command:two", kind: "layout.balance", expectedRevision: 41 })
    ).toBe(true);

    actions.completeCommand("command:one");
    expect(selectPendingWindowManagerCommand(store.getState())?.id).toBe("command:two");

    actions.completeCommand("command:two", {
      code: "layout_balanced",
      message: "The layout is balanced.",
      severity: "info",
      field: null,
    });
    expect(selectPendingWindowManagerCommand(store.getState())).toBeNull();
    expect(selectWindowManagerDiagnostic(store.getState())).toMatchObject({
      code: "layout_balanced",
      severity: "info",
    });

    actions.clearDiagnostic();
    expect(selectWindowManagerDiagnostic(store.getState())).toBeNull();
    expect(store.getState().actions).toBe(actions);
  });
});
