import { describe, expect, it } from "vitest";

import { resolveActiveWorkspace } from "../active-workspace";
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

const HOME = "/Users/operator";
const home = workspace({ id: "ws_home", root_dir: HOME, name: "home" });
const alpha = workspace({ id: "ws_alpha", name: "alpha" });
const beta = workspace({ id: "ws_beta", name: "beta" });

describe("resolveActiveWorkspace", () => {
  it("Should hide the operator-home registration from project lists", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [home, alpha],
      userHomeDir: HOME,
      scope: "workspace",
      selectedWorkspaceId: "ws_alpha",
    });
    expect(resolved.projectWorkspaces.map(item => item.id)).toEqual(["ws_alpha"]);
    expect(resolved.homeWorkspace?.id).toBe("ws_home");
    expect(resolved.scope).toBe("workspace");
    expect(resolved.chip).toEqual({ name: "alpha", monogram: "AL" });
  });

  it("Should keep Global on with a ~ chip when no project workspaces exist", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [home],
      userHomeDir: HOME,
      scope: "workspace",
      selectedWorkspaceId: "ws_home",
    });
    expect(resolved.scope).toBe("global");
    expect(resolved.toggleLocked).toBe(true);
    expect(resolved.activeWorkspace).toBeUndefined();
    expect(resolved.chip).toEqual({ name: "Global", monogram: "~" });
    expect(resolved.runtimeWorkspaceId).toBe("ws_home");
  });

  it("Should stay Global when the remembered project is missing instead of falling back", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [home, alpha, beta],
      userHomeDir: HOME,
      scope: "workspace",
      selectedWorkspaceId: "ws_missing",
    });
    expect(resolved.scope).toBe("global");
    expect(resolved.activeWorkspaceId).toBeNull();
    expect(resolved.rememberedWorkspace).toBeUndefined();
    expect(resolved.canDisableGlobal).toBe(false);
    expect(resolved.chip).toEqual({ name: "Global", monogram: "~" });
  });

  it("Should remember the project workspace while Global is on", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [home, alpha],
      userHomeDir: HOME,
      scope: "global",
      selectedWorkspaceId: "ws_alpha",
    });
    expect(resolved.scope).toBe("global");
    expect(resolved.rememberedWorkspace?.id).toBe("ws_alpha");
    expect(resolved.canDisableGlobal).toBe(true);
    expect(resolved.chip).toEqual({ name: "Global", monogram: "~" });
  });

  it("Should claim nothing while resolution is pending", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [home, alpha],
      userHomeDir: undefined,
      scope: "workspace",
      selectedWorkspaceId: "ws_alpha",
      pending: true,
    });
    expect(resolved.pending).toBe(true);
    // Loading is not "deleted": no home row leaking into projects.
    expect(resolved.projectWorkspaces).toEqual([]);
    expect(resolved.activeWorkspaceId).toBeNull();
    expect(resolved.runtimeWorkspaceId).toBeNull();
    expect(resolved.toggleLocked).toBe(true);
    expect(resolved.canDisableGlobal).toBe(false);
    expect(resolved.scope).toBe("workspace");
  });

  it("Should keep the Global chip identity while pending in Global scope", () => {
    const resolved = resolveActiveWorkspace({
      workspaces: [],
      userHomeDir: undefined,
      scope: "global",
      selectedWorkspaceId: null,
      pending: true,
    });
    expect(resolved.chip).toEqual({ name: "Global", monogram: "~" });
  });

  it("Should match the home row across trailing separators", () => {
    const trailingHome = workspace({ id: "ws_home", root_dir: `${HOME}/`, name: "home" });
    const resolved = resolveActiveWorkspace({
      workspaces: [trailingHome, alpha],
      userHomeDir: HOME,
      scope: "workspace",
      selectedWorkspaceId: "ws_alpha",
    });
    expect(resolved.homeWorkspace?.id).toBe("ws_home");
    expect(resolved.projectWorkspaces.map(item => item.id)).toEqual(["ws_alpha"]);
  });
});
