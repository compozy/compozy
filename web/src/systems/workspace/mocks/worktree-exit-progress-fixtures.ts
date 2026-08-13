import type { WorktreeExitProgress } from "../lib/worktree-exit-progress";
import type { WorktreeExitCleanupEvidence } from "../types";

/**
 * Exit-progress and cleanup-evidence fixtures.
 *
 * Every model here is the shape `reduceWorktreeExitEvent` folds out of real
 * frames: the phase list is announced once on `started` and never grows, hook
 * chunks stay in arrival order, and a terminal frame carries the captured
 * output plus at most one CTA.
 */
const OPERATION_ID = "op_7f21c9";

function progress(overrides: Partial<WorktreeExitProgress>): WorktreeExitProgress {
  return {
    operationID: OPERATION_ID,
    action: "commit_push",
    phases: [],
    status: "running",
    ...overrides,
  };
}

/** The `started` frame: every phase is on screen before any of them runs. */
export const exitProgressAnnouncedFixture = progress({
  phases: [
    { phase: "commit", state: "pending" },
    { phase: "push", state: "pending" },
  ],
});

/** Mid-run: the commit hook is printing, and stderr is marked as such. */
export const exitProgressAdvancingFixture = progress({
  phases: [
    {
      phase: "commit",
      state: "running",
      chunks: [
        { stream: "stdout", text: "husky > pre-commit\n" },
        { stream: "stdout", text: "oxlint … 214 files, 0 warnings\n" },
        { stream: "stderr", text: "oxfmt: reformatted worktree-status-strip.tsx\n" },
      ],
    },
    { phase: "push", state: "pending" },
  ],
});

/** Done: past-tense labels, the skipped phase says why, and one next step. */
export const exitProgressSuccessFixture = progress({
  action: "commit_push",
  status: "completed",
  phases: [
    {
      phase: "commit",
      state: "completed",
      sha: "4b91ce7",
      output:
        "[pedro/payments-retry 4b91ce7] Retry failed payment captures\n 6 files changed, 142 insertions(+), 18 deletions(-)",
    },
    { phase: "push", state: "skipped", reason: "already up to date" },
  ],
  cta: { action: "open_pr", label: "Open PR" },
});

/** A failure belongs to the phase that produced it, with that phase's output. */
export const exitProgressFailureFixture = progress({
  action: "commit_push",
  status: "failed",
  failedPhase: "push",
  message: "push rejected",
  phases: [
    { phase: "commit", state: "completed", sha: "4b91ce7" },
    {
      phase: "push",
      state: "failed",
      output:
        "! [rejected]        pedro/payments-retry -> pedro/payments-retry (fetch first)\nerror: failed to push some refs to 'github.com:compozy/compozy.git'\nremote rejected",
    },
  ],
});

/**
 * A hook refused the commit. The commit dialog has already closed, so this
 * report is the only place the streamed hook output lands.
 */
export const exitProgressHookFailureFixture = progress({
  action: "commit_push",
  status: "failed",
  failedPhase: "commit",
  message: "hook rejected",
  phases: [
    {
      phase: "commit",
      state: "failed",
      output: "hook stdout\nhook stderr\n",
      chunks: [
        { stream: "stdout", text: "hook stdout\n" },
        { stream: "stderr", text: "hook stderr\n" },
      ],
    },
    { phase: "push", state: "pending" },
  ],
});

/**
 * Cleanup evidence.
 *
 * `safe` is the daemon's verdict and the only thing that offers the action; a
 * blocker suppresses it entirely, and a downgrade is information rather than
 * alarm.
 */
export const cleanupClosedWithoutMergeFixture: WorktreeExitCleanupEvidence = {
  safe: false,
  forge_state: "closed",
  source: "local",
  blocker: "1 commits exist nowhere else.",
};

/** No forge in the picture: the evidence is local and states what it checked. */
export const cleanupLocalSafeFixture: WorktreeExitCleanupEvidence = {
  safe: true,
  source: "local",
  summary: "No commits unique to this branch.",
};

export const cleanupRemoteDowngradeFixture: WorktreeExitCleanupEvidence = {
  safe: true,
  downgraded: true,
  source: "local",
  summary: "Unique commits are already on a remote branch.",
};

export const cleanupStaleRemoteFixture: WorktreeExitCleanupEvidence = {
  forge_state: "merged",
  stale: true,
  safe: false,
  source: "local",
  blocker: "1 commits exist nowhere else.",
};
