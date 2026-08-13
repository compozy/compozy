// Suite: worktree cleanup evidence
// Invariant: stale forge evidence is visibly timestamped and can never authorize cleanup.
// Boundary IN: daemon cleanup evidence rendered by the worktree detail surface.
// Boundary OUT: forge freshness calculation, owned by the backend ExitPlan suite.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { WorktreeMergedEvidence } from "../worktree-merged-evidence";

describe("WorktreeMergedEvidence", () => {
  it("Should mark stale forge evidence without exposing cleanup", () => {
    render(
      <WorktreeMergedEvidence
        cleanup={{
          forge_state: "merged",
          stale: true,
          safe: false,
          source: "local",
          blocker: "1 commits exist nowhere else.",
        }}
        onCleanUp={vi.fn()}
        staleLabel="5 minutes ago"
      />
    );

    expect(screen.getByText(/as of 5 minutes ago/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Clean up" })).not.toBeInTheDocument();
    expect(screen.getByTestId("worktree-merged-evidence")).toHaveAttribute("data-stale");
  });
});
