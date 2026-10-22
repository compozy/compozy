// Suite: session create binding
// Invariant: the create request binds exactly one destination — a project create requires the
// project id (`workspace`), a create outside a project starts in User (`scope: "user"`, never
// the home path or a hidden home registration), and a project without an id yields null so the
// submit withholds.
// Boundary IN: the pure binding function. Boundary OUT: the HTTP adapter and dialog wiring.

import { describe, expect, it } from "vitest";

import { sessionCreateBinding, userSessionBinding } from "../session-create-binding";

describe("sessionCreateBinding", () => {
  it("Should bind the project workspace id in a project", () => {
    expect(sessionCreateBinding({ scope: "project", projectWorkspaceId: "ws_alpha" })).toEqual({
      workspace: "ws_alpha",
    });
  });

  it("Should withhold a project create without a project id", () => {
    expect(sessionCreateBinding({ scope: "project", projectWorkspaceId: null })).toBeNull();
  });

  it("Should start in User outside a project, ignoring any remembered project", () => {
    expect(sessionCreateBinding({ scope: "user", projectWorkspaceId: "ws_alpha" })).toEqual({
      scope: "user",
    });
    expect(userSessionBinding()).toEqual({ scope: "user" });
  });
});
