// Suite: window-manager interaction store
// Invariants: WM-WEB-007/008 — Query remains the snapshot authority, while seam previews,
// command acceptance, and transition-intent reconciliation derive atomically in store transitions.
// Boundary IN: named XState Store events, guarded transitions, and selector subscriptions.
// Boundary OUT: Query snapshots, daemon commands, and pointer transport.
import { act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  createWindowManagerStore,
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
  type WindowManagerDiagnostic,
  windowManagerStore,
} from "../window-manager-store";
import { beginWindowManagerCommand } from "../window-manager-store-commands";

const WORK_AREA = { rect: { x: 0, y: 0, w: 1440, h: 820 }, origin: { x: 0, y: 0 } };
const SEAM_PREVIEW = { splitId: "split:one", boundaryIndex: 0, deltaPx: 20 };
const CONFLICT_DIAGNOSTIC: WindowManagerDiagnostic = {
  code: "layout_revision_conflict",
  message: "The layout changed before the command completed.",
  severity: "warning",
  field: "expected_revision",
};

function state(store: ReturnType<typeof createWindowManagerStore>) {
  return store.getSnapshot().context;
}

function resetWindowManagerStoreSingleton(): void {
  windowManagerStore.trigger.bindingUnbound();
  windowManagerStore.trigger.seamPreviewCleared();
}

describe("window manager store", () => {
  beforeEach(resetWindowManagerStoreSingleton);
  afterEach(() => {
    act(resetWindowManagerStoreSingleton);
  });

  it("Should preserve work area while resetting scoped interaction for a new binding", () => {
    const store = createWindowManagerStore();
    store.trigger.bindingBound({
      binding: { workspaceId: "workspace:alpha", profileId: "marketing", clientId: "client:one" },
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
    store.trigger.seamPreviewSet({ preview: SEAM_PREVIEW });
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
      binding: { workspaceId: "workspace:beta", profileId: "marketing", clientId: "client:two" },
    });

    expect(selectWindowManagerBinding(state(store))).toEqual({
      workspaceId: "workspace:beta",
      profileId: "marketing",
      clientId: "client:two",
    });
    expect(selectWindowManagerConnectionStatus(state(store))).toBe("idle");
    expect(selectWindowManagerWorkArea(state(store))).toEqual(WORK_AREA);
    expect(selectWindowManagerOverlay(state(store))).toBeNull();
    expect(selectDesktopOverviewSegmentRequest(state(store))).toBeNull();
    expect(selectDesktopTransitionIntent(state(store))).toBeNull();
    expect(state(store).seamPreview).toBeNull();
    expect(selectPendingWindowManagerCommand(state(store))).toBeNull();
    expect(selectWindowManagerConflict(state(store))).toBeNull();
    expect(selectWindowManagerDiagnostic(state(store))).toBeNull();
  });

  it("Should preserve transient state when the identical binding is announced again", () => {
    const store = createWindowManagerStore();
    const binding = {
      workspaceId: "workspace:alpha",
      profileId: "marketing",
      clientId: "client:one",
    };
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

  it("Should reset binding-scoped interaction when only the profile changes", () => {
    const store = createWindowManagerStore();
    store.trigger.bindingBound({
      binding: {
        workspaceId: "workspace:alpha",
        profileId: "marketing",
        clientId: "client:one",
      },
    });
    store.trigger.connectionStatusChanged({ status: "connected" });
    store.trigger.overlayOpened({
      overlay: { kind: "layout-editor", desktopId: "desktop:one" },
    });

    store.trigger.bindingBound({
      binding: {
        workspaceId: "workspace:alpha",
        profileId: "research",
        clientId: "client:one",
      },
    });

    expect(selectWindowManagerBinding(state(store))).toEqual({
      workspaceId: "workspace:alpha",
      profileId: "research",
      clientId: "client:one",
    });
    expect(selectWindowManagerConnectionStatus(state(store))).toBe("idle");
    expect(selectWindowManagerOverlay(state(store))).toBeNull();
  });

  it("Should open the overview on a desktop menu action and drop it with the overview", () => {
    const store = createWindowManagerStore();
    store.trigger.overviewSegmentRequested({
      request: { direction: "later", hiddenDesktopIds: ["desktop:six"], anchorDesktopId: "a" },
    });

    store.trigger.overviewDesktopActionRequested({
      request: { desktopId: "desktop:build", action: "close" },
    });
    expect(selectWindowManagerOverlay(state(store))).toEqual({ kind: "desktops-overview" });
    expect(selectDesktopOverviewDesktopAction(state(store))).toEqual({
      desktopId: "desktop:build",
      action: "close",
    });
    // One entry reason at a time: the action replaces the overflow request and the reverse.
    expect(selectDesktopOverviewSegmentRequest(state(store))).toBeNull();

    // The card applied it: the overview stays open and the action is gone, so a remount of that
    // card does not apply it again.
    store.trigger.overviewDesktopActionConsumed();
    expect(selectWindowManagerOverlay(state(store))).toEqual({ kind: "desktops-overview" });
    expect(selectDesktopOverviewDesktopAction(state(store))).toBeNull();

    store.trigger.overlayClosed();
    expect(selectWindowManagerOverlay(state(store))).toBeNull();
    expect(selectDesktopOverviewDesktopAction(state(store))).toBeNull();

    store.trigger.overviewDesktopActionRequested({
      request: { desktopId: "desktop:build", action: "rename" },
    });
    store.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } });
    expect(selectDesktopOverviewDesktopAction(state(store))).toBeNull();
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
      binding: { workspaceId: "workspace:alpha", profileId: "marketing", clientId: "client:one" },
    });
    store.trigger.paletteViewStackSet({ stack: seeded });
    seeded.push({ viewId: "sessions" as const });
    expect(state(store).paletteViewStack).toHaveLength(1);

    store.trigger.bindingBound({
      binding: { workspaceId: "workspace:beta", profileId: "marketing", clientId: "client:two" },
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
    const binding = {
      workspaceId: "workspace:alpha",
      profileId: "marketing",
      clientId: "client:one",
    };
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
      binding: { workspaceId: "workspace:old", profileId: "marketing", clientId: "client:old" },
      toDesktopId: "desktop:four",
    });
    store.trigger.transitionIntentRejected({ binding, toDesktopId: "desktop:three" });

    expect(selectDesktopTransitionIntent(state(store))).toEqual(currentIntent);

    store.trigger.transitionIntentRejected({ binding, toDesktopId: "desktop:four" });
    expect(selectDesktopTransitionIntent(state(store))).toBeNull();
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

  it("Should keep the state identity when a seam preview repeats and clear it once", () => {
    const store = createWindowManagerStore();
    store.trigger.seamPreviewSet({ preview: SEAM_PREVIEW });
    const before = store.getSnapshot();

    store.trigger.seamPreviewSet({ preview: { ...SEAM_PREVIEW } });
    expect(store.getSnapshot()).toBe(before);
    expect(state(store).seamPreview).toEqual(SEAM_PREVIEW);

    store.trigger.seamPreviewSet({ preview: { ...SEAM_PREVIEW, deltaPx: 32 } });
    expect(state(store).seamPreview).toEqual({ ...SEAM_PREVIEW, deltaPx: 32 });

    store.trigger.seamPreviewCleared();
    const cleared = store.getSnapshot();
    expect(state(store).seamPreview).toBeNull();
    store.trigger.seamPreviewCleared();
    expect(store.getSnapshot()).toBe(cleared);
  });
});
