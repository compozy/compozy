// Suite: session workspace switch confirm action
// Invariant: confirming a foreign deep link selects the owning project workspace, but a
// Global-owned (operator-home) session turns Global scope on instead — the home row id must
// never be written into the remembered selection.
// Boundary IN: the confirm action and the active-workspace store.
// Boundary OUT: the route decision component and dialog rendering.

import { beforeEach, describe, expect, it, vi } from "vitest";

import { confirmSessionWorkspaceSwitch } from "../-session-workspace-switch-action";
import {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  setActiveWorkspaceId,
  setActiveWorktreeId,
} from "@/systems/workspace";

function context() {
  return activeWorkspaceStore.getSnapshot().context;
}

describe("confirmSessionWorkspaceSwitch", () => {
  beforeEach(() => {
    clearActiveWorkspaceSelection();
    setActiveWorkspaceId("ws_project");
    setActiveWorktreeId("shell", "ws_project", "wt_project");
  });

  it("Should select the owning project workspace and re-enter the deep link", () => {
    const reenter = vi.fn();
    confirmSessionWorkspaceSwitch(
      { sessionId: "sess_1", workspaceId: "ws_other", workspaceName: "other" },
      { isGlobal: false },
      reenter
    );
    expect(context()).toEqual({
      scope: "workspace",
      selectedWorkspaceId: "ws_other",
      worktreeByScope: {},
    });
    expect(reenter).toHaveBeenCalledOnce();
  });

  it("Should turn Global on for a home-owned session without poisoning the selection", () => {
    const reenter = vi.fn();
    confirmSessionWorkspaceSwitch(
      { sessionId: "sess_1", workspaceId: "ws_home", workspaceName: "operator" },
      { isGlobal: true },
      reenter
    );
    expect(context()).toEqual({
      scope: "global",
      selectedWorkspaceId: "ws_project",
      worktreeByScope: { shell: "wt_project" },
    });
    expect(reenter).toHaveBeenCalledOnce();
  });
});
