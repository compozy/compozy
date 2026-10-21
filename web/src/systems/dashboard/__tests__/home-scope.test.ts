// Suite: Home dashboard scope for the menubar destination
// Invariant: All projects reads whole-system aggregates (empty workspace param); a project
// reads its id; User reads `@user` with global tasks and never the aggregate; an unsettled
// destination returns null.
import { describe, expect, it } from "vitest";

import { homeScopeForActiveWorkspace } from "../lib/home-scope";

describe("homeScopeForActiveWorkspace", () => {
  it("Should map All projects to the empty workspace param", () => {
    const scope = homeScopeForActiveWorkspace({ kind: "all" });
    expect(scope).not.toBeNull();
    expect(scope?.dataScope).toEqual({ kind: "all" });
    expect(scope?.workspaceParam).toBe("");
    expect(scope?.taskScope).toEqual({ scope: "global" });
  });

  it("Should scope a project workspace by its id", () => {
    const scope = homeScopeForActiveWorkspace({ kind: "project", workspaceId: "ws-proj" });
    expect(scope?.workspaceParam).toBe("ws-proj");
    expect(scope?.taskScope).toEqual({ scope: "workspace", workspace: "ws-proj" });
  });

  it("Should read User through @user, never the aggregate", () => {
    const scope = homeScopeForActiveWorkspace({ kind: "user" });
    expect(scope?.dataScope).toEqual({ kind: "user" });
    expect(scope?.workspaceParam).toBe("@user");
    expect(scope?.taskScope).toEqual({ scope: "global" });
  });

  it("Should return null while the destination is unsettled", () => {
    expect(homeScopeForActiveWorkspace(null)).toBeNull();
  });
});
