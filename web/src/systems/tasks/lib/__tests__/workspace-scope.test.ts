import { describe, expect, it } from "vitest";

import { taskScopeForActiveWorkspace } from "../workspace-scope";

describe("taskScopeForActiveWorkspace", () => {
  it("Should bind the home workspace to global task scope", () => {
    expect(
      taskScopeForActiveWorkspace({ id: "ws_home", root_dir: "/Users/operator" }, "/Users/operator")
    ).toEqual({ scope: "global" });
  });

  it("Should bind project workspaces to their workspace task scope", () => {
    expect(
      taskScopeForActiveWorkspace(
        { id: "ws_alpha", root_dir: "/workspace/alpha" },
        "/Users/operator"
      )
    ).toEqual({ scope: "workspace", workspace: "ws_alpha" });
  });

  it("Should withhold task queries until an active workspace exists", () => {
    expect(taskScopeForActiveWorkspace(null, "/Users/operator")).toBeNull();
  });

  it("Should withhold task queries until the user home directory is known", () => {
    expect(
      taskScopeForActiveWorkspace({ id: "ws_home", root_dir: "/Users/operator" }, undefined)
    ).toBeNull();
  });
});
