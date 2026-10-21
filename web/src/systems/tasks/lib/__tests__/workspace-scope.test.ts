// Suite: task scope for the menubar destination
// Invariant: a project binds its id (and only then a worktree); User binds `scope=global`
// (tasks with no workspace), All projects keeps that population until it gains its own
// aggregate; an unsettled destination withholds the query.
import { describe, expect, it } from "vitest";

import { taskScopeForActiveWorkspace } from "../workspace-scope";

const PROJECT = { kind: "project", workspaceId: "ws_alpha" } as const;

describe("taskScopeForActiveWorkspace", () => {
  it("Should bind User focus to global task scope", () => {
    expect(taskScopeForActiveWorkspace({ kind: "user" })).toEqual({ scope: "global" });
  });

  it("Should keep All projects on global task scope", () => {
    expect(taskScopeForActiveWorkspace({ kind: "all" })).toEqual({ scope: "global" });
  });

  it("Should bind a focused project to its id", () => {
    expect(taskScopeForActiveWorkspace(PROJECT)).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
    });
  });

  it("Should attach a worktree only alongside its project workspace", () => {
    expect(taskScopeForActiveWorkspace(PROJECT, "wt_payments")).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
      worktree: "wt_payments",
    });
    expect(taskScopeForActiveWorkspace({ kind: "user" }, "wt_payments")).toEqual({
      scope: "global",
    });
    expect(taskScopeForActiveWorkspace({ kind: "all" }, "wt_payments")).toEqual({
      scope: "global",
    });
  });

  it("Should omit a blank or cleared worktree selection", () => {
    expect(taskScopeForActiveWorkspace(PROJECT, null)).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
    });
    expect(taskScopeForActiveWorkspace(PROJECT, "  ")).not.toHaveProperty("worktree");
  });

  it("Should withhold task queries until the destination settles", () => {
    expect(taskScopeForActiveWorkspace(null)).toBeNull();
    expect(taskScopeForActiveWorkspace(undefined)).toBeNull();
  });
});
