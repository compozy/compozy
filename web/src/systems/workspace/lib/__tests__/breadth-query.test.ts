// Suite: breadth query encoders
// Invariant: a DataScope plus an optional Project filter becomes one route-agnostic query; each
// route spells it its own way — sessions send `all_workspaces` (+ `workspace_ids`) or one
// `workspace_id` (`@user` for User), tasks send `all_workspaces` or `scope=global` for User,
// automations narrow a project to its own definitions, aggregate-only routes (worktrees,
// terminals, agents, suggestions) have params only under All projects, and Loop lists read
// the flat aggregate route or one workspace path (`@user` for User). The filter narrows only
// All projects, and no encoder ever sends `global` or an empty workspace.
// Owning layer: workspace lib. Canonical suite: this file.
import { describe, expect, it } from "vitest";

import {
  aggregateBreadthParams,
  automationBreadthParams,
  breadthQuery,
  loopBreadthTarget,
  sessionCatalogBreadthParams,
  taskBreadthParams,
  USER_SCOPE,
} from "../breadth-query";

describe("breadthQuery", () => {
  it("Should narrow All projects to the filtered projects and User", () => {
    const query = breadthQuery({ kind: "all" }, ["ws_a", USER_SCOPE, " ws_a ", "", "global"]);

    expect(query.workspaceIds).toEqual(["ws_a", "@user"]);
    expect(sessionCatalogBreadthParams(query)).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_a", "@user"],
    });
    expect(taskBreadthParams(query)).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_a", "@user"],
    });
  });

  it("Should read every project when the filter is empty", () => {
    const query = breadthQuery({ kind: "all" }, []);

    expect(query.workspaceIds).toBeUndefined();
    expect(sessionCatalogBreadthParams(query)).toEqual({ all_workspaces: true });
  });

  it("Should ignore the filter outside All projects", () => {
    const query = breadthQuery({ kind: "project", workspaceId: "ws_a" }, ["ws_b"]);

    expect(query.workspaceIds).toBeUndefined();
    expect(sessionCatalogBreadthParams(query)).toEqual({ workspace_id: "ws_a" });
    expect(taskBreadthParams(query)).toEqual({ scope: "workspace", workspace: "ws_a" });
  });

  it("Should spell User as @user for sessions and scope=global for tasks", () => {
    const query = breadthQuery({ kind: "user" });

    expect(sessionCatalogBreadthParams(query)).toEqual({ workspace_id: "@user" });
    expect(taskBreadthParams(query)).toEqual({ scope: "global" });
  });

  it("Should list only a project's automations and User's as scope=global", () => {
    expect(automationBreadthParams(breadthQuery({ kind: "project", workspaceId: "ws_a" }))).toEqual(
      { scope: "workspace", workspace_id: "ws_a" }
    );
    expect(automationBreadthParams(breadthQuery({ kind: "user" }))).toEqual({ scope: "global" });
    expect(automationBreadthParams(breadthQuery({ kind: "all" }, ["ws_a", USER_SCOPE]))).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_a", "@user"],
    });
  });

  it("Should send aggregate params only under All projects", () => {
    expect(aggregateBreadthParams(breadthQuery({ kind: "all" }))).toEqual({ all_workspaces: true });
    expect(aggregateBreadthParams(breadthQuery({ kind: "all" }, ["ws_b"]))).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_b"],
    });
    expect(aggregateBreadthParams(breadthQuery({ kind: "user" }))).toBeNull();
    expect(
      aggregateBreadthParams(breadthQuery({ kind: "project", workspaceId: "ws_a" }))
    ).toBeNull();
  });

  it("Should send Loop lists to the aggregate route under All projects, else one workspace path", () => {
    expect(loopBreadthTarget(breadthQuery({ kind: "all" }, ["ws_a", USER_SCOPE]))).toEqual({
      kind: "aggregate",
      params: { all_workspaces: true, workspace_ids: ["ws_a", "@user"] },
    });
    expect(loopBreadthTarget(breadthQuery({ kind: "all" }))).toEqual({
      kind: "aggregate",
      params: { all_workspaces: true },
    });
    expect(loopBreadthTarget(breadthQuery({ kind: "project", workspaceId: "ws_a" }))).toEqual({
      kind: "workspace",
      workspaceId: "ws_a",
    });
    expect(loopBreadthTarget(breadthQuery({ kind: "user" }))).toEqual({
      kind: "workspace",
      workspaceId: "@user",
    });
  });
});
