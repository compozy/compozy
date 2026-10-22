// Suite: task-route breadth for a data scope
// Invariant: All projects reads the `all_workspaces` aggregate (narrowed only by its Project
// filter), User reads its unbound `scope=global` population, a project binds its id (and only
// then a worktree); the inbox and scheduler backlog keep their own aggregate spellings; an
// unsettled destination withholds the query.
import { describe, expect, it } from "vitest";

import {
  schedulerBacklogScope,
  taskInboxScope,
  taskScopeForActiveWorkspace,
} from "../workspace-scope";

const PROJECT = { kind: "project", workspaceId: "ws_alpha" } as const;

describe("taskScopeForActiveWorkspace", () => {
  it("Should bind User focus to the unbound task scope", () => {
    expect(taskScopeForActiveWorkspace({ kind: "user" })).toEqual({ scope: "global" });
  });

  it("Should read All projects through the all_workspaces aggregate", () => {
    expect(taskScopeForActiveWorkspace({ kind: "all" })).toEqual({ all_workspaces: true });
  });

  it("Should narrow All projects by its Project filter, User included", () => {
    expect(taskScopeForActiveWorkspace({ kind: "all" }, null, ["ws_alpha", "@user"])).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_alpha", "@user"],
    });
    expect(taskScopeForActiveWorkspace({ kind: "all" }, null, [])).toEqual({
      all_workspaces: true,
    });
  });

  it("Should ignore a Project filter outside All projects", () => {
    expect(taskScopeForActiveWorkspace(PROJECT, null, ["ws_beta"])).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
    });
    expect(taskScopeForActiveWorkspace({ kind: "user" }, null, ["ws_beta"])).toEqual({
      scope: "global",
    });
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
      all_workspaces: true,
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

describe("taskInboxScope", () => {
  it("Should omit the scope under unfiltered All projects, since the inbox omission is every task", () => {
    expect(taskInboxScope({ all_workspaces: true })).toEqual({
      scope: undefined,
      workspace: undefined,
      worktree: undefined,
    });
  });

  it("Should carry the All projects Project filter as workspace_ids", () => {
    expect(taskInboxScope({ all_workspaces: true, workspace_ids: ["ws_alpha", "@user"] })).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_alpha", "@user"],
    });
  });

  it("Should keep a bounded scope and its worktree", () => {
    expect(
      taskInboxScope({ scope: "workspace", workspace: "ws_alpha", worktree: "wt_payments" })
    ).toEqual({ scope: "workspace", workspace: "ws_alpha", worktree: "wt_payments" });
    expect(taskInboxScope({ scope: "global" })).toMatchObject({ scope: "global" });
  });
});

describe("schedulerBacklogScope", () => {
  it("Should spell All projects as scope=all and keep bounded scopes", () => {
    expect(schedulerBacklogScope({ all_workspaces: true })).toEqual({ scope: "all" });
    expect(schedulerBacklogScope({ scope: "workspace", workspace: "ws_alpha" })).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
    });
    expect(schedulerBacklogScope({ scope: "global" })).toEqual({
      scope: "global",
      workspace: undefined,
    });
  });
});
