// Suite: breadth query encoders
// Invariant: a DataScope plus an optional Project filter becomes one route-agnostic query; each
// route spells it its own way — sessions send `all_workspaces` (+ `workspace_ids`) or one
// `workspace_id` (`@user` for User), tasks send `all_workspaces` or `scope=global` for User. The
// filter narrows only All projects, and no encoder ever sends `global` or an empty workspace.
// Owning layer: workspace lib. Canonical suite: this file.
import { describe, expect, it } from "vitest";

import {
  breadthQuery,
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
});
