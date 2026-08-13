// Suite: active-workspace store transitions
// Invariant: the persisted scope contract — selecting a workspace turns Global off in one
// gesture, leaving Global requires a remembered project, clearing resets to Global, and the
// toggle flips relative to the *resolved* scope, never the stored one.
// Boundary IN: the module-scoped @xstate/store and its imperative API.
// Boundary OUT: resolution (lib/active-workspace) and persistence envelope (use-workspaces suite).

import { beforeEach, describe, expect, it } from "vitest";

import {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  disableGlobalScope,
  enableGlobalScope,
  setActiveWorkspaceId,
  toggleGlobalScope,
} from "../active-workspace-store";

function context() {
  return activeWorkspaceStore.getSnapshot().context;
}

describe("activeWorkspaceStore", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
  });

  it("Should turn Global off and select the workspace in one gesture", () => {
    enableGlobalScope();
    setActiveWorkspaceId("ws_alpha");
    expect(context()).toEqual({ scope: "workspace", selectedWorkspaceId: "ws_alpha" });
  });

  it("Should remember the selected workspace while Global is on", () => {
    setActiveWorkspaceId("ws_alpha");
    enableGlobalScope();
    expect(context()).toEqual({ scope: "global", selectedWorkspaceId: "ws_alpha" });
    disableGlobalScope();
    expect(context()).toEqual({ scope: "workspace", selectedWorkspaceId: "ws_alpha" });
  });

  it("Should refuse to leave Global without a remembered workspace", () => {
    expect(context().selectedWorkspaceId).toBeNull();
    disableGlobalScope();
    expect(context().scope).toBe("global");
  });

  it("Should reset to Global when the selection is cleared", () => {
    setActiveWorkspaceId("ws_alpha");
    clearActiveWorkspaceSelection();
    expect(context()).toEqual({ scope: "global", selectedWorkspaceId: null });
  });

  it("Should toggle relative to the resolved scope, not the stored one", () => {
    // Stored scope says workspace, but the resolver coerced to Global (e.g. the
    // remembered project was deleted): the toggle must treat Global as current,
    // and without a disable path it stays a no-op.
    setActiveWorkspaceId("ws_gone");
    toggleGlobalScope({ scope: "global", canDisable: false });
    expect(context()).toEqual({ scope: "workspace", selectedWorkspaceId: "ws_gone" });

    toggleGlobalScope({ scope: "workspace", canDisable: true });
    expect(context().scope).toBe("global");
    toggleGlobalScope({ scope: "global", canDisable: true });
    expect(context()).toEqual({ scope: "workspace", selectedWorkspaceId: "ws_gone" });
  });
});
