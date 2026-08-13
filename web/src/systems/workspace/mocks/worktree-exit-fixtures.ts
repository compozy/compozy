import type { WorktreeExitPlanPayload, WorktreeStatusPayload } from "../types";

/**
 * Exit-plan fixtures.
 *
 * Every variant is a payload the daemon can actually produce: the ladder
 * positions differ only by which action is `primary` and which rows are
 * enabled, and absence (no forge, no upstream) is expressed by leaving fields
 * out rather than by adding a disabled placeholder.
 */
const COMMIT_SCOPE: WorktreeExitPlanPayload["commit_scope"] = {
  changed_files: 6,
  insertions: 142,
  deletions: 18,
  untracked_files: ["docs/notes.md", "internal/worktree/probe_test.go"],
  untracked_total: 2,
};

export const forgeCapabilitiesFixture = {
  provider: "github",
  request_noun: "Pull request",
  open_action_label: "Open pull request",
  view_action_label: "View pull request",
  supports_draft: true,
  default_branch: "main",
} as NonNullable<WorktreeExitPlanPayload["forge"]>;

function plan(overrides: Partial<WorktreeExitPlanPayload>): WorktreeExitPlanPayload {
  return {
    worktree_id: "wt_payments_retry",
    actions: [],
    commit_scope: COMMIT_SCOPE,
    cleanup: { safe: false },
    base: "main",
    ...overrides,
  } as WorktreeExitPlanPayload;
}

/** Dirty tree: Commit leads, everything downstream states why it cannot run. */
export const exitPlanCommitFixture = plan({
  primary: "commit",
  forge: forgeCapabilitiesFixture,
  actions: [
    { action: "commit", label: "Commit", enabled: true },
    { action: "commit_push", label: "Commit & push", enabled: true },
    {
      action: "push",
      label: "Push",
      enabled: false,
      blocked_reason: "Commit changes before pushing branches.",
    },
    {
      action: "open_pr",
      label: "Open pull request",
      enabled: false,
      blocked_reason: "Push commits before opening pull requests.",
    },
  ],
});

/** Clean tree with unpushed commits: Push leads and publishes a new branch. */
export const exitPlanPushFixture = plan({
  primary: "push",
  forge: forgeCapabilitiesFixture,
  commit_scope: {
    ...COMMIT_SCOPE,
    changed_files: 0,
    insertions: 0,
    deletions: 0,
    untracked_files: [],
    untracked_total: 0,
  },
  actions: [
    {
      action: "commit",
      label: "Commit",
      enabled: false,
      blocked_reason: "No uncommitted changes to commit.",
    },
    { action: "push", label: "Push", enabled: true, publish: true },
    {
      action: "open_pr",
      label: "Open pull request",
      enabled: false,
      blocked_reason: "Push commits before opening pull requests.",
    },
  ],
});

export const exitPlanOpenPrFixture = plan({
  primary: "open_pr",
  forge: forgeCapabilitiesFixture,
  actions: [
    {
      action: "commit",
      label: "Commit",
      enabled: false,
      blocked_reason: "No uncommitted changes to commit.",
    },
    { action: "push", label: "Push", enabled: false, blocked_reason: "No commits pending push." },
    { action: "open_pr", label: "Open pull request", enabled: true },
  ],
});

export const exitPlanViewPrFixture = plan({
  primary: "view_pr",
  forge: forgeCapabilitiesFixture,
  forge_status: {
    provider: "github",
    pr_number: 412,
    pr_state: "open",
    pr_url: "https://github.com/compozy/compozy/pull/412",
    merged: false,
    fetched_at: "2026-08-12T10:00:00Z",
  },
  actions: [
    {
      action: "commit",
      label: "Commit",
      enabled: false,
      blocked_reason: "No uncommitted changes to commit.",
    },
    {
      action: "view_pr",
      label: "View pull request",
      enabled: true,
      pr_number: 412,
      url: "https://github.com/compozy/compozy/pull/412",
    },
  ],
});

/** A bound session is running: one cause blocks every row. */
export const exitPlanPausedFixture = plan({
  primary: "commit",
  forge: forgeCapabilitiesFixture,
  global_pause_cause: "Git actions are paused while a bound session is running.",
  actions: [
    {
      action: "commit",
      label: "Commit",
      enabled: false,
      blocked_reason: "Git actions are paused while a bound session is running.",
    },
    {
      action: "push",
      label: "Push",
      enabled: false,
      blocked_reason: "Git actions are paused while a bound session is running.",
    },
  ],
});

export const exitPlanStatusUnreadableFixture = plan({
  primary: "commit",
  global_pause_cause: "Git status could not be read for this worktree.",
  actions: [
    {
      action: "commit",
      label: "Commit",
      enabled: false,
      blocked_reason: "Git status could not be read for this worktree.",
    },
  ],
});

export const exitPlanDivergedFixture = plan({
  primary: "commit",
  forge: forgeCapabilitiesFixture,
  actions: [
    { action: "commit", label: "Commit", enabled: true },
    {
      action: "push",
      label: "Push",
      enabled: false,
      blocked_reason: "Branch has diverged from upstream. Rebase/merge first.",
    },
  ],
});

/** No remote configured: the daemon states why, and no PR row exists at all. */
export const exitPlanNoRemoteFixture = plan({
  primary: "commit",
  actions: [
    { action: "commit", label: "Commit", enabled: true },
    {
      action: "commit_push",
      label: "Commit & push",
      enabled: false,
      blocked_reason: 'Add an "origin" remote before continuing.',
    },
  ],
});

/** Zero credential: no forge serves the remote, so only the compare link remains. */
export const exitPlanZeroCredentialFixture = plan({
  primary: "commit",
  browser_url: "https://github.com/compozy/compozy/compare/main...pedro/payments-retry",
  actions: [
    { action: "commit", label: "Commit", enabled: true },
    {
      action: "open_pr",
      label: "Open in browser",
      enabled: true,
      url: "https://github.com/compozy/compozy/compare/main...pedro/payments-retry",
    },
  ],
});

export const exitPlanMergedFixture = plan({
  primary: "view_pr",
  forge: forgeCapabilitiesFixture,
  forge_status: {
    provider: "github",
    pr_number: 398,
    pr_state: "merged",
    pr_url: "https://github.com/compozy/compozy/pull/398",
    merged: true,
    fetched_at: "2026-08-12T09:00:00Z",
  },
  cleanup: {
    safe: true,
    forge_state: "merged",
    source: "forge",
    summary: "Merged on github · safe to clean up",
  },
  actions: [
    {
      action: "view_pr",
      label: "View pull request",
      enabled: true,
      pr_number: 398,
      url: "https://github.com/compozy/compozy/pull/398",
    },
  ],
});

export const exitPlanCleanupBlockedFixture = plan({
  primary: "commit",
  cleanup: {
    safe: false,
    blocker: "3 unpushed commits would be lost.",
    summary: "Not safe to clean up yet",
  },
  actions: [{ action: "commit", label: "Commit", enabled: true }],
});

export const worktreeStatusDirtyFixture: WorktreeStatusPayload = {
  worktree_id: "wt_payments_retry",
  status: {
    branch: "pedro/payments-retry",
    detached: false,
    head_sha: "9ec3d15a2f",
    dirty_files: 6,
    insertions: 142,
    deletions: 18,
    has_upstream: true,
    ahead: 3,
    ahead_of_base: 3,
    behind: 0,
    refreshed_at: "2026-08-12T10:05:00Z",
  },
  forge: null,
} as WorktreeStatusPayload;

/** Unknown is unknown: no counts, no upstream, nothing inferred. */
export const worktreeStatusUnknownFixture: WorktreeStatusPayload = {
  worktree_id: "wt_bench_harness",
  status: {
    branch: "pedro/bench-harness",
    detached: false,
    head_sha: null,
    dirty_files: null,
    insertions: null,
    deletions: null,
    has_upstream: false,
    ahead: null,
    ahead_of_base: null,
    behind: null,
    read_error: "git status failed: not a git repository",
    refreshed_at: null,
  },
  forge: null,
} as WorktreeStatusPayload;
