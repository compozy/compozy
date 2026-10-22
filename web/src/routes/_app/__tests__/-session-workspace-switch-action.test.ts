// Suite: session workspace switch confirm action
// Invariant: confirming a foreign deep link focuses the owning destination — the owning
// project, or User for a session without one — and re-enters the link. Focusing User
// keeps the remembered project and its window picks; the empty owner id is never written
// into the remembered selection.
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
    setActiveWorktreeId("window:1", "ws_project", "wt_project");
  });

  it("Should focus the owning project workspace and re-enter the deep link", () => {
    const reenter = vi.fn();
    confirmSessionWorkspaceSwitch(
      { sessionId: "sess_1", workspaceId: "ws_other", workspaceName: "other" },
      reenter
    );
    expect(context()).toMatchObject({
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_other" },
      selectedWorkspaceId: "ws_other",
    });
    expect(reenter).toHaveBeenCalledOnce();
  });

  it("Should focus User for a session without a project and keep the remembered project", () => {
    const reenter = vi.fn();
    confirmSessionWorkspaceSwitch(
      { sessionId: "sess_1", workspaceId: "", workspaceName: "" },
      reenter
    );
    expect(context()).toMatchObject({
      breadthMode: "focused",
      focus: { kind: "user" },
      selectedWorkspaceId: "ws_project",
      worktreeByScope: { "window:1": "wt_project" },
    });
    expect(reenter).toHaveBeenCalledOnce();
  });
});
