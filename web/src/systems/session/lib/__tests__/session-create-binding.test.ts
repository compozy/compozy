// Suite: session create binding
// Invariant: the create request binds `workspace` xor `workspace_path` — a workspace-scoped
// create requires the project id, a Global create prefers the home registration id over the raw
// home path, and an unresolvable destination yields null so the submit withholds.
// Boundary IN: the pure binding function. Boundary OUT: the HTTP adapter and dialog wiring.

import { describe, expect, it } from "vitest";

import { sessionCreateBinding } from "../session-create-binding";

describe("sessionCreateBinding", () => {
  it("Should bind the project workspace id in workspace scope", () => {
    expect(
      sessionCreateBinding({
        scope: "workspace",
        projectWorkspaceId: "ws_alpha",
        homeWorkspaceId: "ws_home",
        userHomeDir: "/Users/operator",
      })
    ).toEqual({ workspace: "ws_alpha" });
  });

  it("Should withhold a workspace-scoped create without a project id", () => {
    expect(
      sessionCreateBinding({
        scope: "workspace",
        projectWorkspaceId: null,
        homeWorkspaceId: "ws_home",
        userHomeDir: "/Users/operator",
      })
    ).toBeNull();
  });

  it("Should prefer the home registration id over the home path in Global scope", () => {
    expect(
      sessionCreateBinding({
        scope: "global",
        projectWorkspaceId: "ws_alpha",
        homeWorkspaceId: "ws_home",
        userHomeDir: "/Users/operator",
      })
    ).toEqual({ workspace: "ws_home" });
  });

  it("Should fall back to the home path when Global has no registration yet", () => {
    expect(
      sessionCreateBinding({
        scope: "global",
        projectWorkspaceId: null,
        homeWorkspaceId: undefined,
        userHomeDir: "/Users/operator",
      })
    ).toEqual({ workspace_path: "/Users/operator" });
  });

  it("Should yield null when nothing resolves the Global destination", () => {
    expect(
      sessionCreateBinding({
        scope: "global",
        projectWorkspaceId: null,
        homeWorkspaceId: undefined,
        userHomeDir: undefined,
      })
    ).toBeNull();
  });
});
