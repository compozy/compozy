import { describe, expect, it } from "vitest";

import { taskScopeForActiveWorkspace } from "../workspace-scope";

describe("taskScopeForActiveWorkspace", () => {
  it("Should bind Global menubar scope to global task scope", () => {
    expect(taskScopeForActiveWorkspace("global", "ws_alpha")).toEqual({ scope: "global" });
  });

  it("Should bind workspace menubar scope to the selected project id", () => {
    expect(taskScopeForActiveWorkspace("workspace", "ws_alpha")).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
    });
  });

  it("Should withhold task queries until workspace scope has a project id", () => {
    expect(taskScopeForActiveWorkspace("workspace", null)).toBeNull();
    expect(taskScopeForActiveWorkspace(null)).toBeNull();
  });
});
