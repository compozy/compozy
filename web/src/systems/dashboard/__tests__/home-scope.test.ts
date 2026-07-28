import { describe, expect, it } from "vitest";

import { homeScopeForActiveWorkspace } from "../lib/home-scope";

const HOME_DIR = "/Users/tester";

function workspace(id: string, rootDir: string) {
  return { id, root_dir: rootDir, name: id };
}

describe("homeScopeForActiveWorkspace", () => {
  it("Should map the home workspace to the global scope with an empty param", () => {
    const scope = homeScopeForActiveWorkspace(workspace("ws-home", HOME_DIR), HOME_DIR);
    expect(scope).not.toBeNull();
    expect(scope?.workspaceParam).toBe("");
    expect(scope?.taskScope).toEqual({ scope: "global" });
  });

  it("Should scope a project workspace by its id", () => {
    const scope = homeScopeForActiveWorkspace(
      workspace("ws-proj", "/Users/tester/dev/proj"),
      HOME_DIR
    );
    expect(scope?.workspaceParam).toBe("ws-proj");
    expect(scope?.taskScope).toEqual({ scope: "workspace", workspace: "ws-proj" });
  });

  it("Should return null while resolution is incomplete, never the global scope", () => {
    // Undetermined must NOT collapse to global: that let a project workspace
    // issue whole-system reads before its home dir arrived. Callers gate on null.
    expect(homeScopeForActiveWorkspace(undefined, HOME_DIR)).toBeNull();
    expect(homeScopeForActiveWorkspace(workspace("ws", "/x"), undefined)).toBeNull();
  });
});
