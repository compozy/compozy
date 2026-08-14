// Suite: active-workspace store transitions
// Invariant: the persisted scope contract — selecting a workspace turns Global off in one
// gesture, leaving Global requires a remembered project, clearing resets to Global, and the
// toggle flips relative to the resolved scope. Worktree selections are per-window, while a
// scope without an explicit pick inherits the shell selection. An explicit root remains local.
// Boundary IN: the module-scoped @xstate/store and its imperative API.
// Boundary OUT: resolution (lib/active-workspace) and persistence envelope (use-workspaces suite).

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { useActiveWorktree } from "../../hooks/use-active-worktree";
import {
  worktreeListingFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
} from "../../mocks/worktree-fixtures";
import {
  ACTIVE_WORKSPACE_PERSIST_KEY,
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  disableGlobalScope,
  enableGlobalScope,
  pruneWorktreeScopes,
  setActiveWorkspaceId,
  setActiveWorktreeId,
  SHELL_WORKTREE_SCOPE,
  toggleGlobalScope,
} from "../active-workspace-store";

const WORKSPACE = "ws_launch_hq";
const WINDOW_ONE = "window:one";
const WINDOW_TWO = "window:two";

function context() {
  return activeWorkspaceStore.getSnapshot().context;
}

/** Resolution is a read over store + listing; it needs a renderer for the selector. */
function resolve(scopeId: string, listing = worktreeListingFixture) {
  return renderHook(() => useActiveWorktree(scopeId, listing)).result.current;
}

describe("activeWorkspaceStore", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
  });

  it("Should turn Global off and select the workspace in one gesture", () => {
    enableGlobalScope();
    setActiveWorkspaceId("ws_alpha");
    expect(context()).toEqual({
      scope: "workspace",
      selectedWorkspaceId: "ws_alpha",
      worktreeByScope: {},
    });
  });

  it("Should remember the selected workspace while Global is on", () => {
    setActiveWorkspaceId("ws_alpha");
    enableGlobalScope();
    expect(context()).toEqual({
      scope: "global",
      selectedWorkspaceId: "ws_alpha",
      worktreeByScope: {},
    });
    disableGlobalScope();
    expect(context()).toEqual({
      scope: "workspace",
      selectedWorkspaceId: "ws_alpha",
      worktreeByScope: {},
    });
  });

  it("Should refuse to leave Global without a remembered workspace", () => {
    expect(context().selectedWorkspaceId).toBeNull();
    disableGlobalScope();
    expect(context().scope).toBe("global");
  });

  it("Should reset to Global when the selection is cleared", () => {
    setActiveWorkspaceId("ws_alpha");
    clearActiveWorkspaceSelection();
    expect(context()).toEqual({
      scope: "global",
      selectedWorkspaceId: null,
      worktreeByScope: {},
    });
  });

  it("Should toggle relative to the resolved scope, not the stored one", () => {
    // Stored scope says workspace, but the resolver coerced to Global (e.g. the
    // remembered project was deleted): the toggle must treat Global as current,
    // and without a disable path it stays a no-op.
    setActiveWorkspaceId("ws_gone");
    toggleGlobalScope({ scope: "global", canDisable: false });
    expect(context()).toEqual({
      scope: "workspace",
      selectedWorkspaceId: "ws_gone",
      worktreeByScope: {},
    });

    toggleGlobalScope({ scope: "workspace", canDisable: true });
    expect(context().scope).toBe("global");
    toggleGlobalScope({ scope: "global", canDisable: true });
    expect(context()).toEqual({
      scope: "workspace",
      selectedWorkspaceId: "ws_gone",
      worktreeByScope: {},
    });
  });
});

describe("active workspace worktree scope", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
    window.localStorage.clear();
  });

  it("Should persist the selected workspace", () => {
    setActiveWorkspaceId(WORKSPACE);

    expect(window.localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY)).toContain(WORKSPACE);
  });

  it("Should keep two window scopes independently selectable", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");

    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_payments_retry");
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_auth_refresh");
    expect(context().selectedWorkspaceId).toBe(WORKSPACE);
  });

  it("Should clear one scope without touching its sibling", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");

    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, null);

    expect(context().worktreeByScope[WINDOW_ONE]).toBeNull();
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_auth_refresh");
  });

  it("Should drop every worktree scope when the workspace changes", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");

    setActiveWorkspaceId("ws_risk_ops");

    expect(context().worktreeByScope).toEqual({});
  });

  it("Should rebase the selection when a worktree from another workspace is picked", () => {
    setActiveWorkspaceId(WORKSPACE);
    setActiveWorktreeId(WINDOW_ONE, "ws_risk_ops", "wt_other");

    expect(context().selectedWorkspaceId).toBe("ws_risk_ops");
    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_other");
  });

  it("Should turn Global off when a worktree is selected", () => {
    enableGlobalScope();
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");

    expect(context().scope).toBe("workspace");
    expect(context().selectedWorkspaceId).toBe(WORKSPACE);
    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_payments_retry");
  });

  it("Should prune scopes for windows that no longer exist", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");

    pruneWorktreeScopes([WINDOW_TWO]);

    expect(context().worktreeByScope[WINDOW_ONE]).toBeUndefined();
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_auth_refresh");
  });

  it("Should keep the shell scope through a prune", () => {
    setActiveWorktreeId(SHELL_WORKTREE_SCOPE, WORKSPACE, "wt_payments_retry");

    pruneWorktreeScopes([]);

    expect(context().worktreeByScope[SHELL_WORKTREE_SCOPE]).toBe("wt_payments_retry");
  });

  it("Should make every selection gesture the shell ambient default", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");

    expect(context().worktreeByScope[SHELL_WORKTREE_SCOPE]).toBe("wt_payments_retry");
  });

  it("Should root only the acting scope when the active workspace is re-selected", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");

    setActiveWorkspaceId(WORKSPACE, { scopeId: WINDOW_ONE });

    expect(context().worktreeByScope[WINDOW_ONE]).toBeNull();
    expect(context().worktreeByScope[SHELL_WORKTREE_SCOPE]).toBeNull();
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_auth_refresh");
  });

  it("Should preserve worktree scopes on a programmatic re-selection", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");

    setActiveWorkspaceId(WORKSPACE);

    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_payments_retry");
    expect(context().worktreeByScope[SHELL_WORKTREE_SCOPE]).toBe("wt_payments_retry");
  });
});

describe("worktree selection resolution", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
  });

  it("Should resolve a ready worktree as the active scope", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);

    const selection = resolve(WINDOW_ONE);

    expect(selection.activeWorktree?.id).toBe(worktreeReadyDirtyRunningFixture.id);
    expect(selection.fallback).toBeNull();
  });

  it("Should fall back to the parent when the selected worktree went missing", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, worktreeMissingFixture.id);

    const selection = resolve(WINDOW_ONE);

    expect(selection.activeWorktree).toBeNull();
    expect(selection.fallback?.reason).toBe("missing");
    expect(selection.selectedWorktreeId).toBe(worktreeMissingFixture.id);
  });

  it("Should fall back when the selected worktree is not ready yet", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, worktreePendingFixture.id);

    const selection = resolve(WINDOW_ONE);

    expect(selection.activeWorktree).toBeNull();
    expect(selection.fallback?.reason).toBe("unavailable");
  });

  it("Should fall back when the selection left the list entirely", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_removed_by_someone_else");

    const selection = resolve(WINDOW_ONE);

    expect(selection.activeWorktree).toBeNull();
    expect(selection.fallback?.reason).toBe("missing");
  });

  it("Should restore a persisted selection while the worktree is still present", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);

    expect(resolve(WINDOW_ONE).activeWorktree?.id).toBe(worktreeReadyDirtyRunningFixture.id);
  });

  it("Should report no selection for an unscoped window", () => {
    const selection = resolve(WINDOW_TWO);

    expect(selection.selectedWorktreeId).toBeNull();
    expect(selection.activeWorktree).toBeNull();
    expect(selection.fallback).toBeNull();
    expect(selection.resolved).toBe(true);
  });

  it("Should keep a stored selection unresolved until the live list arrives", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);

    const selection = renderHook(() => useActiveWorktree(WINDOW_ONE, undefined)).result.current;

    expect(selection.selectedWorktreeId).toBe(worktreeReadyDirtyRunningFixture.id);
    expect(selection.activeWorktree).toBeNull();
    expect(selection.fallback).toBeNull();
    expect(selection.resolved).toBe(false);
  });

  it("Should ignore inherited object properties when resolving a scope", () => {
    setActiveWorktreeId(SHELL_WORKTREE_SCOPE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);

    expect(resolve("constructor").selectedWorktreeId).toBe(worktreeReadyDirtyRunningFixture.id);
  });

  it("Should inherit the shell selection in a window with no pick of its own", () => {
    setActiveWorktreeId(SHELL_WORKTREE_SCOPE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);

    const selection = resolve(WINDOW_ONE);

    expect(selection.activeWorktree?.id).toBe(worktreeReadyDirtyRunningFixture.id);
    expect(selection.fallback).toBeNull();
  });

  it("Should keep an explicit pick independent of the shell selection", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);
    setActiveWorktreeId(SHELL_WORKTREE_SCOPE, WORKSPACE, "wt_other_pick");

    expect(resolve(WINDOW_ONE).selectedWorktreeId).toBe(worktreeReadyDirtyRunningFixture.id);
  });

  it("Should let an explicit root override the inherited selection", () => {
    setActiveWorktreeId(SHELL_WORKTREE_SCOPE, WORKSPACE, worktreeReadyDirtyRunningFixture.id);
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, null);

    expect(resolve(WINDOW_ONE).selectedWorktreeId).toBeNull();
    expect(resolve(WINDOW_TWO).selectedWorktreeId).toBe(worktreeReadyDirtyRunningFixture.id);
  });
});
