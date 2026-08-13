import { describe, expect, it } from "vitest";

import { reduceWorktreeExitEvent } from "../worktree-exit-progress";
import type { WorktreeExitEventPayload } from "../worktree-events";

/**
 * Invariant: progress is folded from stream frames only — phases are announced
 * by the daemon and never invented, and output appears when a step reports it.
 * Owning layer: this reducer, which the progress toast renders.
 */
function started(): WorktreeExitEventPayload {
  return { op_id: "op_1", action: "commit_push", phases: ["commit", "push"], state: "running" };
}

describe("reduceWorktreeExitEvent", () => {
  it("Should announce every phase up front as pending", () => {
    const progress = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    expect(progress?.phases).toEqual([
      { phase: "commit", state: "pending" },
      { phase: "push", state: "pending" },
    ]);
    expect(progress?.status).toBe("running");
  });

  it("Should mark a phase running without inventing output for it", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const next = reduceWorktreeExitEvent(start, "worktree.exit_action_step", {
      op_id: "op_1",
      action: "commit_push",
      phase: "commit",
      state: "running",
    });
    const commit = next?.phases.find(phase => phase.phase === "commit");
    expect(commit?.state).toBe("running");
    expect(commit?.output).toBeUndefined();
  });

  it("Should carry a skip reason from the step result", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const next = reduceWorktreeExitEvent(start, "worktree.exit_action_step", {
      op_id: "op_1",
      action: "commit_push",
      phase: "commit",
      state: "skipped",
      result: {
        op_id: "op_1",
        action: "commit_push",
        steps: [{ phase: "commit", state: "skipped", reason: "nothing to commit" }],
      },
    });
    const commit = next?.phases.find(phase => phase.phase === "commit");
    expect(commit?.state).toBe("skipped");
    expect(commit?.reason).toBe("nothing to commit");
  });

  it("Should attribute a mid-chain failure to its own phase and keep the message", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const failed = reduceWorktreeExitEvent(start, "worktree.exit_action_failed", {
      op_id: "op_1",
      action: "commit_push",
      phase: "push",
      state: "failed",
      message: "remote rejected",
      result: {
        op_id: "op_1",
        action: "commit_push",
        steps: [
          { phase: "commit", state: "completed", sha: "abc1234" },
          { phase: "push", state: "failed", output: "! [rejected]" },
        ],
      },
    });
    expect(failed?.status).toBe("failed");
    expect(failed?.failedPhase).toBe("push");
    expect(failed?.message).toBe("remote rejected");
    expect(failed?.phases.find(phase => phase.phase === "push")?.output).toBe("! [rejected]");
  });

  it("Should carry the daemon's single follow-up action on completion", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const done = reduceWorktreeExitEvent(start, "worktree.exit_action_completed", {
      op_id: "op_1",
      action: "commit_push",
      state: "completed",
      result: {
        op_id: "op_1",
        action: "commit_push",
        steps: [
          { phase: "commit", state: "completed" },
          { phase: "push", state: "completed" },
        ],
        cta: { action: "open_pr", label: "Open pull request" },
      },
    });
    expect(done?.status).toBe("completed");
    expect(done?.cta?.label).toBe("Open pull request");
  });

  it("Should ignore frames belonging to a different operation", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const stale = reduceWorktreeExitEvent(start, "worktree.exit_action_failed", {
      op_id: "op_other",
      action: "commit_push",
      state: "failed",
      message: "should not land",
    });
    expect(stale?.status).toBe("running");
    expect(stale?.message).toBeUndefined();
  });
});

/**
 * Live hook output (`worktree.exit_hook_output`).
 *
 * Invariant: redacted chunks render while the phase is still running, in
 * arrival order, and survive the phase's terminal frame. Owning layer: this
 * reducer, which the progress toast renders.
 */
describe("reduceWorktreeExitEvent · hook output", () => {
  function hookChunk(chunk: string, stream: "stdout" | "stderr" = "stdout") {
    return {
      op_id: "op_1",
      action: "commit_push",
      phase: "commit" as const,
      stream,
      chunk,
    };
  }

  it("Should append chunks to the running phase in arrival order", () => {
    let progress = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    progress = reduceWorktreeExitEvent(progress, "worktree.exit_action_step", {
      op_id: "op_1",
      action: "commit_push",
      phase: "commit",
      state: "running",
    });
    progress = reduceWorktreeExitEvent(
      progress,
      "worktree.exit_hook_output",
      hookChunk("running pre-commit\n")
    );
    progress = reduceWorktreeExitEvent(
      progress,
      "worktree.exit_hook_output",
      hookChunk("lint: 0 issues\n")
    );

    const commit = progress?.phases.find(phase => phase.phase === "commit");
    expect(commit?.state).toBe("running");
    expect(commit?.chunks?.map(entry => entry.text)).toEqual([
      "running pre-commit\n",
      "lint: 0 issues\n",
    ]);
  });

  it("Should keep the stream each chunk came from", () => {
    let progress = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    progress = reduceWorktreeExitEvent(
      progress,
      "worktree.exit_hook_output",
      hookChunk("warning: slow hook\n", "stderr")
    );

    const commit = progress?.phases.find(phase => phase.phase === "commit");
    expect(commit?.chunks?.[0]).toEqual({ stream: "stderr", text: "warning: slow hook\n" });
  });

  it("Should keep streamed chunks when the phase's terminal frame lands", () => {
    let progress = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    progress = reduceWorktreeExitEvent(
      progress,
      "worktree.exit_hook_output",
      hookChunk("pre-commit ok\n")
    );
    progress = reduceWorktreeExitEvent(progress, "worktree.exit_action_step", {
      op_id: "op_1",
      action: "commit_push",
      phase: "commit",
      state: "completed",
      result: {
        op_id: "op_1",
        action: "commit_push",
        steps: [{ phase: "commit", state: "completed", sha: "abc1234" }],
      },
    });

    const commit = progress?.phases.find(phase => phase.phase === "commit");
    expect(commit?.state).toBe("completed");
    expect(commit?.chunks?.map(entry => entry.text)).toEqual(["pre-commit ok\n"]);
  });

  it("Should ignore a hook frame that names no phase, stream or chunk", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const next = reduceWorktreeExitEvent(start, "worktree.exit_hook_output", {
      op_id: "op_1",
      action: "commit_push",
    });
    expect(next?.phases.every(phase => phase.chunks === undefined)).toBe(true);
  });

  it("Should ignore hook output belonging to another operation", () => {
    const start = reduceWorktreeExitEvent(undefined, "worktree.exit_action_started", started());
    const next = reduceWorktreeExitEvent(start, "worktree.exit_hook_output", {
      ...hookChunk("leaked\n"),
      op_id: "op_other",
    });
    expect(next?.phases.every(phase => phase.chunks === undefined)).toBe(true);
  });
});
