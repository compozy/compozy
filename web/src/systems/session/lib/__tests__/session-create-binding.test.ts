// Suite: session create binding
// Invariant: the create request binds `workspace` xor `workspace_path` — a project create
// requires the project id, a create outside a project binds the operator-home path (never a
// hidden home registration), and an unresolvable destination yields null so the submit withholds.
// Boundary IN: the pure binding function. Boundary OUT: the HTTP adapter and dialog wiring.

import { describe, expect, it } from "vitest";

import { sessionCreateBinding } from "../session-create-binding";

describe("sessionCreateBinding", () => {
  it("Should bind the project workspace id in a project", () => {
    expect(
      sessionCreateBinding({
        scope: "project",
        projectWorkspaceId: "ws_alpha",
        userHomeDir: "/Users/operator",
      })
    ).toEqual({ workspace: "ws_alpha" });
  });

  it("Should withhold a project create without a project id", () => {
    expect(
      sessionCreateBinding({
        scope: "project",
        projectWorkspaceId: null,
        userHomeDir: "/Users/operator",
      })
    ).toBeNull();
  });

  it("Should bind the home path outside a project, ignoring any remembered project", () => {
    expect(
      sessionCreateBinding({
        scope: "user",
        projectWorkspaceId: "ws_alpha",
        userHomeDir: "/Users/operator",
      })
    ).toEqual({ workspace_path: "/Users/operator" });
  });

  it("Should yield null when nothing resolves the destination outside a project", () => {
    expect(
      sessionCreateBinding({
        scope: "user",
        projectWorkspaceId: null,
        userHomeDir: undefined,
      })
    ).toBeNull();
  });
});
