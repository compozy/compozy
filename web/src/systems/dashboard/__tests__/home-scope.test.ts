import { describe, expect, it } from "vitest";

import { homeScopeForActiveWorkspace } from "../lib/home-scope";

describe("homeScopeForActiveWorkspace", () => {
  it("Should map Global menubar scope to the empty workspace param", () => {
    const scope = homeScopeForActiveWorkspace("global");
    expect(scope).not.toBeNull();
    expect(scope?.workspaceParam).toBe("");
    expect(scope?.taskScope).toEqual({ scope: "global" });
  });

  it("Should scope a project workspace by its id", () => {
    const scope = homeScopeForActiveWorkspace("workspace", "ws-proj");
    expect(scope?.workspaceParam).toBe("ws-proj");
    expect(scope?.taskScope).toEqual({ scope: "workspace", workspace: "ws-proj" });
  });

  it("Should return null while workspace destination is incomplete", () => {
    expect(homeScopeForActiveWorkspace("workspace", null)).toBeNull();
    expect(homeScopeForActiveWorkspace(undefined)).toBeNull();
  });
});
