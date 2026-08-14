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

  it("Should attach a worktree only alongside its project workspace", () => {
    expect(taskScopeForActiveWorkspace("workspace", "ws_alpha", "wt_payments")).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
      worktree: "wt_payments",
    });
    expect(taskScopeForActiveWorkspace("global", "ws_alpha", "wt_payments")).toEqual({
      scope: "global",
    });
  });

  it("Should omit a blank or cleared worktree selection", () => {
    expect(taskScopeForActiveWorkspace("workspace", "ws_alpha", null)).toEqual({
      scope: "workspace",
      workspace: "ws_alpha",
    });
    expect(taskScopeForActiveWorkspace("workspace", "ws_alpha", "  ")).not.toHaveProperty(
      "worktree"
    );
  });

  it("Should withhold task queries until workspace scope has a project id", () => {
    expect(taskScopeForActiveWorkspace("workspace", null)).toBeNull();
    expect(taskScopeForActiveWorkspace(null)).toBeNull();
  });
});
