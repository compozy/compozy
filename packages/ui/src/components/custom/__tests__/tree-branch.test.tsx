import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TreeBranch, TreeBranchNode } from "../tree-branch";

describe("TreeBranch", () => {
  it("Should indent the rail and draw each node's elbow, closing on the last node", () => {
    render(
      <TreeBranch data-testid="rail">
        <TreeBranchNode data-testid="first">first</TreeBranchNode>
        <TreeBranchNode data-testid="last">last</TreeBranchNode>
      </TreeBranch>
    );

    expect(screen.getByTestId("rail")).toHaveAttribute("data-slot", "tree-branch");
    expect(screen.getByTestId("rail").className).toContain("pl-5");
    const node = screen.getByTestId("first");
    expect(node).toHaveAttribute("data-slot", "tree-branch-node");
    expect(node.className).toContain("before:border-l");
    expect(node.className).toContain("last:before:rounded-bl-md");
    expect(node.className).toContain("[--branch-arm:1rem]");
  });

  it("Should land a header node's elbow higher and let a caller set its own arm", () => {
    render(
      <>
        <TreeBranchNode arm="header" data-testid="header" />
        <TreeBranchNode className="[--branch-arm:0.75rem]" data-testid="custom" />
      </>
    );

    expect(screen.getByTestId("header").className).toContain("[--branch-arm:0.875rem]");
    expect(screen.getByTestId("header").className).not.toContain("[--branch-arm:1rem]");
    expect(screen.getByTestId("custom").className).toContain("[--branch-arm:0.75rem]");
    expect(screen.getByTestId("custom").className).not.toContain("[--branch-arm:1rem]");
  });

  it("Should draw the connector on a caller element and keep the caller's slot", () => {
    render(
      <TreeBranchNode
        data-slot="session-subagent-row"
        render={<li aria-label="child" className="flex" />}
      />
    );

    const node = screen.getByLabelText("child");
    expect(node.tagName).toBe("LI");
    expect(node).toHaveAttribute("data-slot", "session-subagent-row");
    expect(node.className).toContain("flex");
    expect(node.className).toContain("before:border-l");
  });
});
