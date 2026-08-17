// Suite: window-manager interaction store
// Invariants: WM-WEB-007/008 — Query remains the snapshot authority, while placement, gesture,
// command acceptance, and transition-intent reconciliation derive atomically in store transitions.
// Boundary IN: named XState Store events, guarded transitions, and selector subscriptions.
// Boundary OUT: Query snapshots, daemon commands, and pointer transport.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { useWindowManagerGestureActive } from "../../hooks/use-window-manager-store";
import type { SnapTarget } from "../../lib/snap-targets";
import {
  createWindowManagerStore,
  selectDesktopOverviewSegmentRequest,
  selectDesktopTransitionIntent,
  selectPendingWindowManagerCommand,
  selectWindowManagerBinding,
  selectWindowManagerConflict,
  selectWindowManagerConnectionStatus,
  selectWindowManagerDiagnostic,
  selectWindowManagerGesture,
  selectWindowManagerGestureDragging,
  selectWindowManagerGesturePreview,
  selectWindowManagerOverlay,
  selectWindowManagerWorkArea,
  type WindowManagerDiagnostic,
  windowManagerStore,
} from "../window-manager-store";
import {
  advanceWindowManagerPlacementCycle,
  beginWindowManagerCommand,
  finishWindowManagerGesture,
} from "../window-manager-store-commands";

const WORK_AREA = { rect: { x: 0, y: 0, w: 1440, h: 820 }, origin: { x: 0, y: 0 } };
const SNAP_TARGET: SnapTarget = {
  kind: "tile",
  id: "edge:left:0.5",
  edge: "left",
  ratio: 0.5,
  rect: { x: 0, y: 0, w: 596, h: 800 },
  zoneRect: { x: 0, y: 0, w: 600, h: 800 },
};
const CONFLICT_DIAGNOSTIC: WindowManagerDiagnostic = {
  code: "layout_revision_conflict",
  message: "The layout changed before the command completed.",
  severity: "warning",
  field: "expected_revision",
};

function state(store: ReturnType<typeof createWindowManagerStore>) {
  return store.getSnapshot().context;
}

function beginGesture(store: ReturnType<typeof createWindowManagerStore>): void {
  store.trigger.gestureBegan({
    pointerId: 4,
    point: { x: 420, y: 180 },
    workArea: WORK_AREA.rect,
    layoutRevision: 17,
    source: { windowId: "window:tasks", nodeId: null, groupId: null, moveMode: "window" },
  });
}

function resetWindowManagerStoreSingleton(): void {
  windowManagerStore.trigger.bindingUnbound();
  windowManagerStore.trigger.gestureCleared();
  windowManagerStore.trigger.seamPreviewCleared();
}

describe("window manager store", () => {
  beforeEach(resetWindowManagerStoreSingleton);
  afterEach(() => {
    act(resetWindowManagerStoreSingleton);
  });

  it("Should keep Query-owned snapshot data out of the interaction context", () => {
    const store = createWindowManagerStore();
    const context = state(store);

    expect(context).not.toHaveProperty("snapshot");
    expect(context).not.toHaveProperty("revision");
    expect(context).not.toHaveProperty("desktops");
    expect(context).not.toHaveProperty("windows");
    expect(context).not.toHaveProperty("actions");
  });

  it("Should preserve work area while resetting scoped interaction for a new binding", () => {
    const store = createWindowManagerStore();
    store.trigger.bindingBound({
      binding: { workspaceId: "workspace:alpha", clientId: "client:one" },
    });
    store.trigger.workAreaMeasured({ workArea: WORK_AREA });
    store.trigger.connectionStatusChanged({ status: "connected" });
    store.trigger.overviewSegmentRequested({
      request: {
        direction: "later",
        hiddenDesktopIds: ["desktop:three", "desktop:four"],
        anchorDesktopId: "desktop:two",
      },
    });
    store.trigger.transitionIntentChanged({
      intent: {
        fromDesktopId: "desktop:one",
        toDesktopId: "desktop:two",
        direction: "later",
        mode: "slide",
      },
    });
    beginGesture(store);
    expect(
      beginWindowManagerCommand(store, {
        id: "command:one",
        kind: "window.move",
        expectedRevision: 17,
      })
    ).toBe(true);
    store.trigger.conflictRecorded({
      conflict: { commandId: "command:one", expectedRevision: 17, currentRevision: 18 },
      diagnostic: CONFLICT_DIAGNOSTIC,
    });

    store.trigger.bindingBound({
      binding: { workspaceId: "workspace:beta", clientId: "client:two" },
    });

    expect(selectWindowManagerBinding(state(store))).toEqual({
      workspaceId: "workspace:beta",
      clientId: "client:two",
    });
    expect(selectWindowManagerConnectionStatus(state(store))).toBe("disconnected");
    expect(selectWindowManagerWorkArea(state(store))).toEqual(WORK_AREA);
    expect(selectWindowManagerOverlay(state(store))).toBeNull();
    expect(selectDesktopOverviewSegmentRequest(state(store))).toBeNull();
    expect(selectDesktopTransitionIntent(state(store))).toBeNull();
    expect(state(store).placementCycles).toEqual({});
    expect(selectWindowManagerGesture(state(store))).toBeNull();
    expect(selectPendingWindowManagerCommand(state(store))).toBeNull();
    expect(selectWindowManagerConflict(state(store))).toBeNull();
    expect(selectWindowManagerDiagnostic(state(store))).toBeNull();
  });

  it("Should preserve transient state when the identical binding is announced again", () => {
    const store = createWindowManagerStore();
    const binding = { workspaceId: "workspace:alpha", clientId: "client:one" };
    store.trigger.bindingBound({ binding });
    store.trigger.connectionStatusChanged({ status: "connected" });
    store.trigger.overlayOpened({
      overlay: { kind: "layout-editor", desktopId: "desktop:one" },
    });
    const before = store.getSnapshot();

    store.trigger.bindingBound({ binding });

    expect(store.getSnapshot()).toBe(before);
    expect(selectWindowManagerConnectionStatus(state(store))).toBe("connected");
    expect(selectWindowManagerOverlay(state(store))).toEqual({
      kind: "layout-editor",
      desktopId: "desktop:one",
    });
  });

  it("Should keep one exclusive overlay and scope an overflow request to the overview", () => {
    const store = createWindowManagerStore();
    const hiddenDesktopIds = ["desktop:one", "desktop:two"];
    store.trigger.overlayOpened({
      overlay: { kind: "layout-editor", desktopId: "desktop:build" },
    });

    store.trigger.overviewSegmentRequested({
      request: {
        direction: "earlier",
        hiddenDesktopIds,
        anchorDesktopId: "desktop:five",
      },
    });
    hiddenDesktopIds.push("desktop:external-mutation");

    expect(selectWindowManagerOverlay(state(store))).toEqual({ kind: "desktops-overview" });
    expect(selectDesktopOverviewSegmentRequest(state(store))).toEqual({
      direction: "earlier",
      hiddenDesktopIds: ["desktop:one", "desktop:two"],
      anchorDesktopId: "desktop:five",
    });

    store.trigger.overlayClosed();
    expect(selectWindowManagerOverlay(state(store))).toBeNull();
    expect(selectDesktopOverviewSegmentRequest(state(store))).toBeNull();

    store.trigger.overviewSegmentRequested({
      request: {
        direction: "later",
        hiddenDesktopIds: ["desktop:six"],
        anchorDesktopId: "desktop:five",
      },
    });
    store.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } });
    expect(selectDesktopOverviewSegmentRequest(state(store))).toBeNull();

    store.trigger.overlayOpened({
      overlay: { kind: "layout-editor", desktopId: "desktop:build" },
    });
    expect(selectWindowManagerOverlay(state(store))).toEqual({
      kind: "layout-editor",
      desktopId: "desktop:build",
    });
  });

  it("Should keep the palette on one mode, dropping the other whenever either is raised [UT-059]", () => {
    const store = createWindowManagerStore();

    store.trigger.paletteViewPushed({ viewId: "sessions" });
    store.trigger.paletteViewPushed({ viewId: "sessions" });
    expect(state(store).paletteViewStack).toEqual([{ viewId: "sessions" }, { viewId: "sessions" }]);

    // A destination picker raised from a new tab replaces the view path.
    store.trigger.paletteIntentRequested({
      intent: { kind: "destination", windowId: "window:new-tab" },
    });
    expect(state(store).paletteViewStack).toEqual([]);
    expect(state(store).paletteIntent).toEqual({ kind: "destination", windowId: "window:new-tab" });

    // ⌘E in the other direction drops the destination scope.
    store.trigger.paletteViewStackSet({ stack: [{ viewId: "sessions" }] });
    expect(state(store).paletteIntent).toBeNull();
    expect(state(store).paletteViewStack).toEqual([{ viewId: "sessions" }]);

    store.trigger.paletteViewPopped();
    expect(state(store).paletteViewStack).toEqual([]);
    store.trigger.paletteViewPopped();
    expect(state(store).paletteViewStack).toEqual([]);
  });

  it("Should reset the palette view path for a new binding and isolate a seeded stack", () => {
    const store = createWindowManagerStore();
    const seeded = [{ viewId: "sessions" as const }];
    store.trigger.bindingBound({
      binding: { workspaceId: "workspace:alpha", clientId: "client:one" },
    });
    store.trigger.paletteViewStackSet({ stack: seeded });
    seeded.push({ viewId: "sessions" as const });
    expect(state(store).paletteViewStack).toHaveLength(1);

    store.trigger.bindingBound({
      binding: { workspaceId: "workspace:beta", clientId: "client:two" },
    });
    expect(state(store).paletteViewStack).toEqual([]);
  });

  it("Should isolate measured work-area and transition inputs from caller mutation", () => {
    const store = createWindowManagerStore();
    const rect = { x: 10, y: 20, w: 1200, h: 760 };
    const transition = {
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:two",
      direction: "later" as const,
      mode: "slide" as const,
    };

    store.trigger.workAreaMeasured({ workArea: { rect, origin: { x: 0, y: 44 } } });
    const beforeEqualMeasurement = store.getSnapshot();
    store.trigger.workAreaMeasured({
      workArea: { rect: { x: 10, y: 20, w: 1200, h: 760 }, origin: { x: 0, y: 44 } },
    });
    expect(store.getSnapshot()).toBe(beforeEqualMeasurement);
    store.trigger.transitionIntentChanged({ intent: transition });
    rect.x = 900;
    transition.toDesktopId = "desktop:external-mutation";

    expect(selectWindowManagerWorkArea(state(store))).toEqual({
      rect: { x: 10, y: 20, w: 1200, h: 760 },
      origin: { x: 0, y: 44 },
    });
    expect(selectDesktopTransitionIntent(state(store))).toEqual({
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:two",
      direction: "later",
      mode: "slide",
    });
  });

  it("Should reconcile and reject transition intents with atomic target and binding fences", () => {
    const store = createWindowManagerStore();
    const binding = { workspaceId: "workspace:alpha", clientId: "client:one" };
    store.trigger.bindingBound({ binding });
    store.trigger.transitionIntentChanged({
      intent: {
        fromDesktopId: "desktop:one",
        toDesktopId: "desktop:two",
        direction: "later",
        mode: "instant",
      },
    });

    store.trigger.desktopStateObserved({
      activeDesktopId: "desktop:two",
      reconciledIntent: null,
    });
    expect(selectDesktopTransitionIntent(state(store))).toBeNull();

    const currentIntent = {
      fromDesktopId: "desktop:two",
      toDesktopId: "desktop:four",
      direction: "later" as const,
      mode: "slide" as const,
    };
    store.trigger.transitionIntentChanged({ intent: currentIntent });
    store.trigger.desktopStateObserved({
      activeDesktopId: "desktop:three",
      reconciledIntent: {
        fromDesktopId: "desktop:two",
        toDesktopId: "desktop:three",
        direction: "later",
        mode: "crossfade",
      },
    });
    store.trigger.transitionIntentRejected({
      binding: { workspaceId: "workspace:old", clientId: "client:old" },
      toDesktopId: "desktop:four",
    });
    store.trigger.transitionIntentRejected({ binding, toDesktopId: "desktop:three" });

    expect(selectDesktopTransitionIntent(state(store))).toEqual(currentIntent);

    store.trigger.transitionIntentRejected({ binding, toDesktopId: "desktop:four" });
    expect(selectDesktopTransitionIntent(state(store))).toBeNull();
  });

  it("Should cycle placements per window and reset on side or structural target changes", () => {
    const store = createWindowManagerStore();

    expect(advanceWindowManagerPlacementCycle(store, "window:tasks", "left")).toBe(0);
    expect(advanceWindowManagerPlacementCycle(store, "window:tasks", "left")).toBe(1);
    expect(advanceWindowManagerPlacementCycle(store, "window:agents", "left")).toBe(0);
    expect(advanceWindowManagerPlacementCycle(store, "window:tasks", "right")).toBe(0);

    store.trigger.placementTargetTracked({ windowId: "window:tasks", edge: "top" });
    expect(advanceWindowManagerPlacementCycle(store, "window:tasks", "top")).toBe(1);

    store.trigger.placementTargetTracked({ windowId: "window:tasks", edge: null });
    expect(advanceWindowManagerPlacementCycle(store, "window:tasks", "top")).toBe(0);
  });

  it("Should capture a gesture revision and clear its preview on explicit cancellation", () => {
    const store = createWindowManagerStore();
    beginGesture(store);

    const previewTarget: SnapTarget = { ...SNAP_TARGET, rect: { ...SNAP_TARGET.rect } };
    store.trigger.gesturePreviewed({
      point: { x: 1, y: 240 },
      preview: previewTarget,
      currentWorkArea: WORK_AREA.rect,
    });
    previewTarget.rect.x = 500;

    expect(selectWindowManagerGesturePreview(state(store))).toEqual(SNAP_TARGET);
    expect(selectWindowManagerGesture(state(store))).toMatchObject({
      status: "active",
      layoutRevision: 17,
      currentPoint: { x: 1, y: 240 },
    });

    store.trigger.gestureCancelled({ reason: "escape", point: { x: 4, y: 245 } });
    expect(selectWindowManagerGesturePreview(state(store))).toBeNull();
    expect(selectWindowManagerGesture(state(store))).toMatchObject({
      status: "cancelled",
      reason: "escape",
      finalPoint: { x: 4, y: 245 },
      layoutRevision: 17,
    });
  });

  it("Should report gesture dragging only after the pointer leaves the press point", () => {
    const store = createWindowManagerStore();
    beginGesture(store);

    expect(selectWindowManagerGestureDragging(state(store), "window:tasks")).toBe(false);

    store.trigger.gesturePreviewed({
      point: { x: 421, y: 180 },
      preview: null,
      currentWorkArea: WORK_AREA.rect,
    });
    expect(selectWindowManagerGestureDragging(state(store), "window:tasks")).toBe(true);
    expect(selectWindowManagerGestureDragging(state(store), "window:other")).toBe(false);

    store.trigger.gestureCancelled({ reason: "escape" });
    expect(selectWindowManagerGestureDragging(state(store), "window:tasks")).toBe(false);
  });

  it("Should finish through the pure gesture decision without persisting the session", () => {
    const store = createWindowManagerStore();
    beginGesture(store);
    store.trigger.gesturePreviewed({
      point: { x: 2, y: 300 },
      preview: SNAP_TARGET,
      currentWorkArea: WORK_AREA.rect,
    });

    const decision = finishWindowManagerGesture(store, {
      finalPoint: { x: 1, y: 301 },
      finalTarget: SNAP_TARGET,
      currentRevision: 17,
      currentWorkArea: WORK_AREA.rect,
      clientValid: true,
    });

    expect(decision).toMatchObject({
      kind: "commit",
      command: { expectedRevision: 17, finalPoint: { x: 1, y: 301 } },
    });
    expect(selectWindowManagerGesture(state(store))).toBeNull();
    expect(selectWindowManagerGesture(state(createWindowManagerStore()))).toBeNull();
  });

  it("Should serialize pending commands and record conflicts only for the active command", () => {
    const store = createWindowManagerStore();
    expect(
      beginWindowManagerCommand(store, {
        id: "command:one",
        kind: "layout.arrange",
        expectedRevision: 40,
      })
    ).toBe(true);
    expect(
      beginWindowManagerCommand(store, {
        id: "command:two",
        kind: "layout.balance",
        expectedRevision: 40,
      })
    ).toBe(false);

    store.trigger.conflictRecorded({
      conflict: { commandId: "command:stale", expectedRevision: 39, currentRevision: 41 },
      diagnostic: CONFLICT_DIAGNOSTIC,
    });
    expect(selectPendingWindowManagerCommand(state(store))?.id).toBe("command:one");

    store.trigger.conflictRecorded({
      conflict: { commandId: "command:one", expectedRevision: 40, currentRevision: 41 },
      diagnostic: CONFLICT_DIAGNOSTIC,
    });
    expect(selectPendingWindowManagerCommand(state(store))).toBeNull();
    expect(selectWindowManagerConflict(state(store))).toEqual({
      commandId: "command:one",
      expectedRevision: 40,
      currentRevision: 41,
    });
    expect(selectWindowManagerDiagnostic(state(store))).toEqual(CONFLICT_DIAGNOSTIC);

    expect(
      beginWindowManagerCommand(store, {
        id: "command:two",
        kind: "layout.balance",
        expectedRevision: 41,
      })
    ).toBe(false);
    store.trigger.conflictCleared();
    expect(selectWindowManagerConflict(state(store))).toBeNull();
    expect(selectWindowManagerDiagnostic(state(store))).toBeNull();
  });

  it("Should ignore stale completions and expose a terminal command diagnostic", () => {
    const store = createWindowManagerStore();
    expect(
      beginWindowManagerCommand(store, {
        id: "command:two",
        kind: "layout.balance",
        expectedRevision: 41,
      })
    ).toBe(true);

    store.trigger.commandCompleted({ commandId: "command:one" });
    expect(selectPendingWindowManagerCommand(state(store))?.id).toBe("command:two");

    store.trigger.commandCompleted({
      commandId: "command:two",
      diagnostic: {
        code: "layout_balanced",
        message: "The layout is balanced.",
        severity: "info",
        field: null,
      },
    });
    expect(selectPendingWindowManagerCommand(state(store))).toBeNull();
    expect(selectWindowManagerDiagnostic(state(store))).toMatchObject({
      code: "layout_balanced",
      severity: "info",
    });

    store.trigger.diagnosticCleared();
    expect(selectWindowManagerDiagnostic(state(store))).toBeNull();
  });

  it("Should reject an old-binding completion after a new binding starts a command", () => {
    const store = createWindowManagerStore();
    const alpha = { workspaceId: "workspace:alpha", clientId: "client:one" };
    const beta = { workspaceId: "workspace:beta", clientId: "client:two" };
    store.trigger.bindingBound({ binding: alpha });
    expect(
      beginWindowManagerCommand(store, {
        id: "command:alpha",
        kind: "window.move",
        expectedRevision: 17,
      })
    ).toBe(true);
    store.trigger.bindingBound({ binding: beta });
    expect(
      beginWindowManagerCommand(store, {
        id: "command:beta",
        kind: "window.move",
        expectedRevision: 18,
      })
    ).toBe(true);

    store.trigger.commandCompleted({ commandId: "command:beta", binding: alpha });

    expect(selectPendingWindowManagerCommand(state(store))).toMatchObject({ id: "command:beta" });
    store.trigger.commandCompleted({ commandId: "command:beta", binding: beta });
    expect(selectPendingWindowManagerCommand(state(store))).toBeNull();
  });

  it("Should not rerender an unrelated gesture selector for seam-preview updates", () => {
    let renders = 0;
    const hook = renderHook(() => {
      renders += 1;
      return useWindowManagerGestureActive("window:tasks");
    });
    const initialRenders = renders;

    act(() => {
      windowManagerStore.trigger.seamPreviewSet({
        preview: { kind: "split", splitId: "split:one", boundaryIndex: 0, deltaPx: 20 },
      });
    });

    expect(hook.result.current).toBe(false);
    expect(renders).toBe(initialRenders);
  });

  it("Should update only the dragged-window selector when gesture activity changes", () => {
    const dragged = renderHook(() => useWindowManagerGestureActive("window:tasks"));
    const unrelated = renderHook(() => useWindowManagerGestureActive("window:agents"));

    act(() => {
      windowManagerStore.trigger.gestureBegan({
        pointerId: 4,
        point: { x: 420, y: 180 },
        workArea: WORK_AREA.rect,
        layoutRevision: 17,
        source: { windowId: "window:tasks", nodeId: null, groupId: null, moveMode: "window" },
      });
      windowManagerStore.trigger.gesturePreviewed({
        point: { x: 2, y: 300 },
        preview: SNAP_TARGET,
        currentWorkArea: WORK_AREA.rect,
      });
    });

    expect(dragged.result.current).toBe(true);
    expect(unrelated.result.current).toBe(false);
  });
});
