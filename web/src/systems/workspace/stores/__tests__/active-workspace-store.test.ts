// Suite: active-workspace store transitions
// Invariant: the persisted destination contract — the globe flips breadth and the switcher sets
// focus (a project or User); focusing leaves All projects in one gesture, turning All projects
// off always returns to the focus, clearing resets to All projects with User focused, and the
// toggle flips relative to the resolved destination. The v1 binary scope migrates losslessly on
// the same key. Worktree selections are per-window, survive another destination, and are pruned
// only by the partition that claimed them; a scope without an explicit pick inherits the shell
// selection, and an explicit root remains local.
// Boundary IN: the module-scoped @xstate/store, its imperative API, and its persisted blob.
// Boundary OUT: resolution (lib/active-workspace).

import { renderHook } from "@testing-library/react";
import { rehydrateStore } from "@xstate/store/persist";
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
  disableAllProjects,
  enableAllProjects,
  focusUser,
  pruneWorktreeScopes,
  setActiveWorkspaceId,
  setActiveWorktreeId,
  SHELL_WORKTREE_SCOPE,
  toggleAllProjects,
} from "../active-workspace-store";

const WORKSPACE = "ws_launch_hq";
const WINDOW_ONE = "window:one";
const WINDOW_TWO = "window:two";
const WINDOW_USER = "window:user";
const WS_A_PARTITION = `${WORKSPACE}\u0000default`;
const USER_PARTITION = "user\u0000default";

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

  it("Should clear only the removed worktree in the matching workspace", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_first");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_second");
    activeWorkspaceStore.trigger.worktreeRemoved({ workspaceId: "other", worktreeId: "wt_first" });
    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_first");
    activeWorkspaceStore.trigger.worktreeRemoved({
      workspaceId: WORKSPACE,
      worktreeId: "wt_first",
    });
    expect(context().worktreeByScope[WINDOW_ONE]).toBeNull();
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_second");
    expect(context().worktreeByScope[SHELL_WORKTREE_SCOPE]).toBe("wt_second");
  });

  it("Should leave All projects and focus the project in one gesture", () => {
    enableAllProjects();
    setActiveWorkspaceId("ws_alpha");
    expect(context()).toEqual({
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_alpha" },
      selectedWorkspaceId: "ws_alpha",
      worktreeByScope: {},
      worktreePartitionByScope: {},
      legacySessionsScopeApplied: null,
    });
  });

  it("Should remember the focused project while All projects is on", () => {
    setActiveWorkspaceId("ws_alpha");
    enableAllProjects();
    expect(context()).toMatchObject({
      breadthMode: "all",
      focus: { kind: "project", workspaceId: "ws_alpha" },
      selectedWorkspaceId: "ws_alpha",
    });
    disableAllProjects();
    expect(context()).toMatchObject({
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_alpha" },
    });
  });

  it("Should focus User and keep the remembered project", () => {
    setActiveWorkspaceId("ws_alpha");
    focusUser();
    expect(context()).toMatchObject({
      breadthMode: "focused",
      focus: { kind: "user" },
      selectedWorkspaceId: "ws_alpha",
    });
  });

  it("Should return to User when All projects turns off with no project ever focused", () => {
    expect(context().breadthMode).toBe("all");
    disableAllProjects();
    expect(context()).toMatchObject({ breadthMode: "focused", focus: { kind: "user" } });
  });

  it("Should reset to All projects with User focused when the selection is cleared", () => {
    setActiveWorkspaceId("ws_alpha");
    clearActiveWorkspaceSelection();
    expect(context()).toEqual({
      breadthMode: "all",
      focus: { kind: "user" },
      selectedWorkspaceId: null,
      worktreeByScope: {},
      worktreePartitionByScope: {},
      legacySessionsScopeApplied: null,
    });
  });

  // ADR-004: clearing the selection must not let an already-applied legacy
  // revision apply again on the next settings read.
  it("Should keep the observed legacy breadth revision when the selection is cleared", () => {
    activeWorkspaceStore.trigger.legacySessionsScopeObserved({
      revision: 9,
      value: "all-workspaces",
      explicit: true,
    });
    clearActiveWorkspaceSelection();
    disableAllProjects();
    activeWorkspaceStore.trigger.legacySessionsScopeObserved({
      revision: 9,
      value: "all-workspaces",
      explicit: true,
    });
    expect(context()).toMatchObject({ breadthMode: "focused", legacySessionsScopeApplied: 9 });
  });

  it("Should toggle relative to the resolved destination, not the stored one", () => {
    // The stored focus is a deleted project; the resolver already coerced it to
    // User, so turning All projects off lands on User, never a project that is gone.
    setActiveWorkspaceId("ws_gone");
    enableAllProjects();
    toggleAllProjects({ breadthMode: "all", focus: { kind: "user" } });
    expect(context()).toMatchObject({
      breadthMode: "focused",
      focus: { kind: "user" },
      selectedWorkspaceId: "ws_gone",
    });

    toggleAllProjects({ breadthMode: "focused", focus: { kind: "user" } });
    expect(context().breadthMode).toBe("all");
  });

  it("Should return to User when the globe turns off after User was focused", () => {
    setActiveWorkspaceId("ws_alpha");
    focusUser();
    enableAllProjects();
    toggleAllProjects({ breadthMode: "all", focus: { kind: "user" } });
    expect(context()).toMatchObject({ breadthMode: "focused", focus: { kind: "user" } });
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

  it("Should keep window picks and drop the ambient pick when another project is focused", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");

    setActiveWorkspaceId("ws_risk_ops");

    expect(context().worktreeByScope).toEqual({
      [WINDOW_ONE]: "wt_payments_retry",
      [WINDOW_TWO]: "wt_auth_refresh",
    });
  });

  it("Should rebase the selection when a worktree from another workspace is picked", () => {
    setActiveWorkspaceId(WORKSPACE);
    setActiveWorktreeId(WINDOW_ONE, "ws_risk_ops", "wt_other");

    expect(context().selectedWorkspaceId).toBe("ws_risk_ops");
    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_other");
  });

  it("Should leave All projects when a worktree is selected", () => {
    enableAllProjects();
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");

    expect(context().breadthMode).toBe("focused");
    expect(context().selectedWorkspaceId).toBe(WORKSPACE);
    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_payments_retry");
  });

  it("Should prune scopes for windows that closed in the partition that listed them", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");
    pruneWorktreeScopes(WS_A_PARTITION, [WINDOW_ONE, WINDOW_TWO]);

    pruneWorktreeScopes(WS_A_PARTITION, [WINDOW_TWO]);

    expect(context().worktreeByScope[WINDOW_ONE]).toBeUndefined();
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_auth_refresh");
  });

  it("Should never prune a window no snapshot has claimed", () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");

    pruneWorktreeScopes(WS_A_PARTITION, []);

    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_payments_retry");
  });

  it("Should keep the shell scope through a prune", () => {
    setActiveWorktreeId(SHELL_WORKTREE_SCOPE, WORKSPACE, "wt_payments_retry");

    pruneWorktreeScopes(WS_A_PARTITION, []);

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

describe("persisted destination migration (v1 → v2)", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
    window.localStorage.clear();
  });

  async function rehydrateFrom(blob: unknown) {
    window.localStorage.setItem(ACTIVE_WORKSPACE_PERSIST_KEY, JSON.stringify(blob));
    await rehydrateStore(activeWorkspaceStore);
    return context();
  }

  it("Should turn v1 Global with a remembered project into All projects focused on it", async () => {
    const migrated = await rehydrateFrom({
      context: {
        scope: "global",
        selectedWorkspaceId: "ws_alpha",
        desktopWorkspaceId: "ws_alpha",
        worktreeByScope: { [WINDOW_ONE]: "wt_saved", [SHELL_WORKTREE_SCOPE]: "wt_saved" },
      },
      version: 1,
    });

    expect(migrated).toEqual({
      breadthMode: "all",
      focus: { kind: "project", workspaceId: "ws_alpha" },
      selectedWorkspaceId: "ws_alpha",
      worktreeByScope: { [WINDOW_ONE]: "wt_saved", [SHELL_WORKTREE_SCOPE]: "wt_saved" },
      worktreePartitionByScope: {},
      legacySessionsScopeApplied: null,
    });
  });

  it("Should turn v1 Global without a remembered project into All projects focused on User", async () => {
    const migrated = await rehydrateFrom({
      context: { scope: "global", selectedWorkspaceId: null, worktreeByScope: {} },
      version: 1,
    });

    expect(migrated).toMatchObject({
      breadthMode: "all",
      focus: { kind: "user" },
      selectedWorkspaceId: null,
    });
  });

  it("Should turn a v1 workspace scope into the focused project", async () => {
    const migrated = await rehydrateFrom({
      context: {
        scope: "workspace",
        selectedWorkspaceId: "ws_beta",
        desktopWorkspaceId: null,
        worktreeByScope: { [WINDOW_TWO]: null },
      },
      version: 1,
    });

    expect(migrated).toEqual({
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_beta" },
      selectedWorkspaceId: "ws_beta",
      worktreeByScope: { [WINDOW_TWO]: null },
      worktreePartitionByScope: {},
      legacySessionsScopeApplied: null,
    });
  });

  it("Should rewrite the migrated destination under the same key", async () => {
    await rehydrateFrom({
      context: { scope: "workspace", selectedWorkspaceId: "ws_beta", worktreeByScope: {} },
      version: 1,
    });

    focusUser();

    const stored = JSON.parse(window.localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY)!);
    expect(stored.version).toBe(2);
    expect(stored.context).toMatchObject({
      breadthMode: "focused",
      focus: { kind: "user" },
      selectedWorkspaceId: "ws_beta",
    });
  });
});

describe("partition-aware worktree pruning", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
    window.localStorage.clear();
  });

  it("Should keep a project's window picks through User focus, return, and reload", async () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    setActiveWorktreeId(WINDOW_TWO, WORKSPACE, "wt_auth_refresh");
    pruneWorktreeScopes(WS_A_PARTITION, [WINDOW_ONE, WINDOW_TWO]);

    // The User desktop lists only its own windows; the project's are not closed.
    focusUser();
    pruneWorktreeScopes(USER_PARTITION, [WINDOW_USER]);
    setActiveWorkspaceId(WORKSPACE);
    await rehydrateStore(activeWorkspaceStore);

    expect(context().focus).toEqual({ kind: "project", workspaceId: WORKSPACE });
    expect(context().worktreeByScope[WINDOW_ONE]).toBe("wt_payments_retry");
    expect(context().worktreeByScope[WINDOW_TWO]).toBe("wt_auth_refresh");
  });

  it("Should persist the claim so a window closed before reload is still pruned", async () => {
    setActiveWorktreeId(WINDOW_ONE, WORKSPACE, "wt_payments_retry");
    pruneWorktreeScopes(WS_A_PARTITION, [WINDOW_ONE]);
    await rehydrateStore(activeWorkspaceStore);

    pruneWorktreeScopes(WS_A_PARTITION, []);

    expect(context().worktreeByScope[WINDOW_ONE]).toBeUndefined();
  });
});
