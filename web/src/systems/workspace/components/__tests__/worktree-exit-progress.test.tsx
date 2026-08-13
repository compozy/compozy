import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { WorktreeExitProgress } from "../../lib/worktree-exit-progress";
import { WorktreeExitProgress as WorktreeExitProgressToast } from "../worktree-exit-progress";

/**
 * Invariant: hook output is visible while its phase is still running, in the
 * order the daemon emitted it, with stderr distinguishable from stdout.
 * Owning layer: this component, the only renderer of the progress model.
 */
function progress(overrides: Partial<WorktreeExitProgress> = {}): WorktreeExitProgress {
  return {
    operationID: "op_1",
    action: "commit_push",
    status: "running",
    phases: [
      { phase: "commit", state: "pending" },
      { phase: "push", state: "pending" },
    ],
    ...overrides,
  };
}

describe("WorktreeExitProgress", () => {
  it("Should render hook chunks before the phase reaches its terminal step", () => {
    render(
      <WorktreeExitProgressToast
        progress={progress({
          phases: [
            {
              phase: "commit",
              state: "running",
              chunks: [
                { stream: "stdout", text: "running pre-commit\n" },
                { stream: "stdout", text: "lint: 0 issues\n" },
              ],
            },
            { phase: "push", state: "pending" },
          ],
        })}
      />
    );

    const stream = document.querySelector('[data-slot="worktree-exit-hook-stream"]');
    expect(stream).not.toBeNull();
    expect(stream?.textContent).toBe("running pre-commit\nlint: 0 issues\n");
    // The step is still running, so no captured step output exists yet.
    expect(document.querySelector('[data-slot="worktree-exit-output"]')).toBeNull();
  });

  it("Should mark which pipe each chunk came from", () => {
    render(
      <WorktreeExitProgressToast
        progress={progress({
          phases: [
            {
              phase: "commit",
              state: "running",
              chunks: [
                { stream: "stdout", text: "ok\n" },
                { stream: "stderr", text: "warning: slow hook\n" },
              ],
            },
          ],
        })}
      />
    );

    const chunks = document.querySelectorAll("[data-stream]");
    expect(Array.from(chunks).map(node => node.getAttribute("data-stream"))).toEqual([
      "stdout",
      "stderr",
    ]);
  });

  it("Should keep hook output visible after the phase completes", () => {
    render(
      <WorktreeExitProgressToast
        progress={progress({
          status: "completed",
          phases: [
            {
              phase: "commit",
              state: "completed",
              chunks: [{ stream: "stdout", text: "pre-commit ok\n" }],
            },
          ],
        })}
      />
    );

    expect(document.querySelector('[data-slot="worktree-exit-hook-stream"]')?.textContent).toBe(
      "pre-commit ok\n"
    );
  });

  it("Should render no stream block when no hook printed anything", () => {
    render(<WorktreeExitProgressToast progress={progress()} />);

    expect(document.querySelector('[data-slot="worktree-exit-hook-stream"]')).toBeNull();
  });
});
