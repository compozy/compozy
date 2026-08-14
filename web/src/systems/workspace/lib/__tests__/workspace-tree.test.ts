// Suite: workspace tree projection
// Invariant (UT-128): worktrees nest under their parent, never as siblings; nesting depth is fixed
// at 2 so no worktree can host a worktree; a non-git workspace gets no worktree group at all.
// Owning layer: workspace query projection. Canonical suite: this lib test (no prior owner).
import { describe, expect, it } from "vitest";

import {
  discoveredWorktreeFixture,
  nonGitWorktreeListingFixture,
  worktreeListingFixture,
} from "../../mocks/worktree-fixtures";
import { groupWorkspaceTree } from "../workspace-tree";

const HOME_DIR = "/Users/ada";

const workspaces = [
  { id: "ws_launch_hq", name: "launch-hq", root_dir: "/workspaces/northstar-pay/launch-hq" },
  { id: "ws_risk_ops", name: "risk-ops", root_dir: "/workspaces/northstar-pay/risk-ops" },
  { id: "ws_home", name: "home", root_dir: HOME_DIR },
];

describe("groupWorkspaceTree", () => {
  it("Should nest worktrees under their own parent and never as siblings", () => {
    const tree = groupWorkspaceTree(
      workspaces,
      { ws_launch_hq: worktreeListingFixture, ws_risk_ops: nonGitWorktreeListingFixture },
      HOME_DIR
    );

    const hq = tree.find(node => node.workspace.id === "ws_launch_hq");
    const risk = tree.find(node => node.workspace.id === "ws_risk_ops");

    expect(hq?.worktrees.length).toBeGreaterThan(0);
    // Every entry belongs to the parent it is rendered under.
    for (const entry of hq?.worktrees ?? []) {
      if (entry.worktree) expect(entry.worktree.workspace_id).toBe("ws_launch_hq");
    }
    expect(risk?.worktrees).toEqual([]);
    // Nothing about a worktree leaks into the workspace list itself.
    expect(tree.map(node => node.workspace.id)).toEqual(["ws_launch_hq", "ws_risk_ops"]);
  });

  it("Should give a non-git workspace no worktree group at all", () => {
    const tree = groupWorkspaceTree(
      workspaces,
      { ws_risk_ops: nonGitWorktreeListingFixture },
      HOME_DIR
    );

    const risk = tree.find(node => node.workspace.id === "ws_risk_ops");
    // Absent, never an empty-but-present affordance.
    expect(risk?.gitBacked).toBe(false);
    expect(risk?.worktrees).toEqual([]);
    expect(risk?.adoptedCount).toBe(0);
  });

  it("Should treat a workspace with no listing as having no worktree affordance", () => {
    const tree = groupWorkspaceTree(workspaces, {}, HOME_DIR);

    expect(tree.every(node => node.gitBacked === false)).toBe(true);
    expect(tree.every(node => node.worktrees.length === 0)).toBe(true);
  });

  it("Should expose no child slot on a worktree entry, fixing nesting depth at two", () => {
    const tree = groupWorkspaceTree(workspaces, { ws_launch_hq: worktreeListingFixture }, HOME_DIR);
    const entry = tree.find(node => node.workspace.id === "ws_launch_hq")?.worktrees[0];

    // The row model carries no nested collection, so no surface can render a
    // worktree under a worktree without changing this type (US-010 EC-1).
    expect(entry).toBeDefined();
    expect(Object.keys(entry ?? {})).not.toContain("worktrees");
    expect(Object.keys(entry ?? {})).not.toContain("children");
  });

  it("Should count adopted worktrees only, leaving discovered entries out", () => {
    const tree = groupWorkspaceTree(workspaces, { ws_launch_hq: worktreeListingFixture }, HOME_DIR);
    const hq = tree.find(node => node.workspace.id === "ws_launch_hq");

    expect(hq?.adoptedCount).toBe(worktreeListingFixture.worktrees.length);
    // The discovered checkout still renders, it just never enters the count.
    expect(hq?.worktrees.some(entry => entry.name === discoveredWorktreeFixture.name)).toBe(true);
  });

  it("Should hide the operator-home registration from project workspace rows", () => {
    const tree = groupWorkspaceTree(workspaces, {}, HOME_DIR);

    expect(tree.map(node => node.workspace.id)).toEqual(["ws_launch_hq", "ws_risk_ops"]);
  });
});
