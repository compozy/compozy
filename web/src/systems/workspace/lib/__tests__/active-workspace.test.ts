// Suite: active-workspace resolver
// Invariant: the destination model resolves one desktop partition and one DataScope per
// (breadth, focus): a project → its partition and {project}; User → partition `user`,
// {user}, and no runtime workspace; All projects → {all} on the focused project's partition
// (or `global` with User focused). A missing project resolves to User, never to another
// project; zero projects never lock the globe; every registered workspace is a project row.
// Surfaces encode a focused scope as the project id or `@user` (sessions, Loops, terminals)
// and read an item with no workspace as User.
// Owning layer: lib resolver. Boundary OUT: store transitions (active-workspace-store suite).
import { describe, expect, it } from "vitest";

import { resolveActiveWorkspace } from "../active-workspace";
import { dataScopeFromSelector, focusedWorkspaceSelector, ownerDataScope } from "../destination";
import type { WorkspacePayload } from "../../types";

function workspace(
  overrides: Partial<WorkspacePayload> & Pick<WorkspacePayload, "id">
): WorkspacePayload {
  return {
    root_dir: `/workspace/${overrides.id}`,
    add_dirs: [],
    name: overrides.id,
    created_at: "2026-04-06T10:00:00Z",
    updated_at: "2026-04-06T10:00:00Z",
    ...overrides,
  };
}

const alpha = workspace({ id: "ws_alpha", name: "alpha" });
const beta = workspace({ id: "ws_beta", name: "beta" });

describe("resolveActiveWorkspace", () => {
  it("Should bind a focused project's partition and data scope", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [alpha, beta],
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_alpha" },
    });
    expect(resolved.dataScope).toEqual({ kind: "project", workspaceId: "ws_alpha" });
    expect(resolved.desktopWorkspaceId).toBe("ws_alpha");
    expect(resolved.runtimeWorkspaceId).toBe("ws_alpha");
    expect(resolved.chip).toEqual({ kind: "project", name: "alpha", monogram: "AL" });
  });

  it("Should bind the user partition and a User data scope with no runtime workspace", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [alpha],
      breadthMode: "focused",
      focus: { kind: "user" },
    });
    expect(resolved.desktopWorkspaceId).toBe("user");
    expect(resolved.dataScope).toEqual({ kind: "user" });
    expect(resolved.runtimeWorkspaceId).toBeNull();
    expect(resolved.activeWorkspace).toBeUndefined();
    expect(resolved.chip).toEqual({ kind: "user", name: "User", monogram: "~" });
  });

  it("Should keep the focused project's desktop under All projects", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [beta, alpha],
      breadthMode: "all",
      focus: { kind: "project", workspaceId: "ws_alpha" },
    });
    expect(resolved.dataScope).toEqual({ kind: "all" });
    expect(resolved.desktopWorkspaceId).toBe("ws_alpha");
    expect(resolved.runtimeWorkspaceId).toBeNull();
    expect(resolved.rememberedWorkspace?.id).toBe("ws_alpha");
    expect(resolved.chip.kind).toBe("all");
    expect(resolved.chip.name).toBe("All projects");
  });

  it("Should bind the global partition under All projects with User focused", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [beta, alpha],
      breadthMode: "all",
      focus: { kind: "user" },
    });
    expect(resolved.desktopWorkspaceId).toBe("global");
    expect(resolved.desktopWorkspace).toBeUndefined();
  });

  it("Should resolve a missing project to User instead of falling back to another project", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [alpha, beta],
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_missing" },
    });
    expect(resolved.focus).toEqual({ kind: "user" });
    expect(resolved.dataScope).toEqual({ kind: "user" });
    expect(resolved.desktopWorkspaceId).toBe("user");
    expect(resolved.activeWorkspaceId).toBeNull();
  });

  it("Should let zero projects focus User instead of locking All projects", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [],
      breadthMode: "focused",
      focus: { kind: "user" },
    });
    expect(resolved.breadthMode).toBe("focused");
    expect(resolved.dataScope).toEqual({ kind: "user" });
    expect(resolved.desktopWorkspaceId).toBe("user");
  });

  it("Should list every registered workspace as a project, including one at the home folder", () => {
    const homeRow = workspace({ id: "ws_home", root_dir: "/Users/operator", name: "home" });
    const resolved = resolveActiveWorkspace({
      workspaces: [homeRow, alpha],
      breadthMode: "all",
      focus: { kind: "user" },
    });
    expect(resolved.projectWorkspaces.map(item => item.id)).toEqual(["ws_home", "ws_alpha"]);
  });

  it("Should claim nothing while the catalog is pending, except destinations always valid", () => {
    const project = resolveActiveWorkspace({
      workspaces: [],
      breadthMode: "focused",
      focus: { kind: "project", workspaceId: "ws_alpha" },
      pending: true,
    });
    expect(project.pending).toBe(true);
    expect(project.dataScope).toBeNull();
    expect(project.desktopWorkspaceId).toBeNull();
    expect(project.runtimeWorkspaceId).toBeNull();

    const user = resolveActiveWorkspace({
      workspaces: [],
      breadthMode: "focused",
      focus: { kind: "user" },
      pending: true,
    });
    expect(user.dataScope).toEqual({ kind: "user" });
    expect(user.desktopWorkspaceId).toBeNull();
  });
});

describe("destination selectors", () => {
  it("Should encode a focused scope as its project id or @user, and nothing for All projects", () => {
    expect(focusedWorkspaceSelector({ kind: "project", workspaceId: "ws_alpha" })).toBe("ws_alpha");
    expect(focusedWorkspaceSelector({ kind: "user" })).toBe("@user");
    expect(focusedWorkspaceSelector({ kind: "all" })).toBe("");
    expect(focusedWorkspaceSelector(null)).toBe("");
  });

  it("Should decode a selector back to its scope", () => {
    expect(dataScopeFromSelector("@user")).toEqual({ kind: "user" });
    expect(dataScopeFromSelector("ws_alpha")).toEqual({ kind: "project", workspaceId: "ws_alpha" });
    expect(dataScopeFromSelector("")).toBeNull();
  });

  it("Should read an item without a workspace as User, never as every item", () => {
    expect(ownerDataScope("")).toEqual({ kind: "user" });
    expect(ownerDataScope(undefined)).toEqual({ kind: "user" });
    expect(ownerDataScope("ws_alpha")).toEqual({ kind: "project", workspaceId: "ws_alpha" });
  });
});
