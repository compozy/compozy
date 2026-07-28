// Suite: window-manager layout profile transition logic
// Invariant: a destructive profile load stays pending while the current draft has unapplied edits.
// Boundary IN: profile editor store transitions.
// Boundary OUT: React subscriptions, query-cache reconciliation, and layout-profile transport.
import { describe, expect, it } from "vitest";

import { windowManagerLayoutProfileEditorLogic } from "../window-manager-layout-profile-editor-store";
import type { WindowManagerLayoutResourceRecord } from "../../lib/window-manager-layout-types";

const record: WindowManagerLayoutResourceRecord = {
  kind: "window_layout",
  id: "two-up-review",
  version: 4,
  scope: { kind: "global", id: "" },
  spec: {
    version: 1,
    id: "two-up-review",
    displayName: "Two-up review",
    aspectVariant: "landscape",
    participantSlots: [],
    overflowPolicy: "reject",
    document: { version: 2, workspaceId: "", desktops: [], windows: {}, overrides: {} },
  },
  createdAt: "2026-07-22T00:00:00Z",
  updatedAt: "2026-07-22T00:00:00Z",
};

describe("windowManagerLayoutProfileEditorLogic", () => {
  it("holds a pending destructive load", () => {
    const store = windowManagerLayoutProfileEditorLogic.createStore();
    const [snapshot] = store.transition(store.getInitialSnapshot(), {
      type: "loadRequested",
      draftDirty: true,
      record,
    });

    expect(snapshot.context.pendingLoad).toEqual(record);
    expect(snapshot.context.selected).toBeNull();
  });
});
