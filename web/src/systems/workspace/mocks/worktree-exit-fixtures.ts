import type { WorktreeExitPlanPayload, WorktreeStatusPayload } from "../types";

/**
 * Exit-plan fixtures.
 *
 * Every variant is a payload `buildExitActions` / `selectExitPrimary` can
 * actually produce. Labels, blocked reasons, and row sets are daemon literals.
 */

const REASON_NO_CHANGES = "No uncommitted changes to commit.";
const REASON_COMMIT_FIRST = "Commit changes before pushing branches.";
const REASON_NO_PENDING = "No commits pending push.";
const REASON_PUSH_FIRST = "Push commits before opening pull requests.";
const REASON_DIVERGED = "Branch has diverged from upstream. Rebase/merge first.";
const REASON_NO_PR = "No branch changes to include in a PR.";
const REASON_PAUSE = "Git actions are paused while a bound session is running.";
const REASON_UNREADABLE = "Git status could not be read for this worktree.";

const COMPARE_URL =
  "https://github.com/compozy/compozy/compare/main...pedro%2Fpayments-retry?expand=1";
const PR_412_URL = "https://github.com/compozy/compozy/pull/412";
const PR_398_URL = "https://github.com/compozy/compozy/pull/398";

type ExitAction = WorktreeExitPlanPayload["actions"][number];

const COMMIT_ENABLED: ExitAction = { action: "commit", label: "Commit", enabled: true };
const COMMIT_NO_CHANGES: ExitAction = {
  action: "commit",
  label: "Commit",
  enabled: false,
  blocked_reason: REASON_NO_CHANGES,
};
const COMMIT_PAUSE: ExitAction = {
  action: "commit",
  label: "Commit",
  enabled: false,
  blocked_reason: REASON_PAUSE,
};
const COMMIT_UNREADABLE: ExitAction = {
  action: "commit",
  label: "Commit",
  enabled: false,
  blocked_reason: REASON_UNREADABLE,
};

const COMMIT_PUSH_ENABLED: ExitAction = {
  action: "commit_push",
  label: "Commit & push",
  enabled: true,
};
const COMMIT_PUSH_NO_CHANGES: ExitAction = {
  action: "commit_push",
  label: "Commit & push",
  enabled: false,
  blocked_reason: REASON_NO_CHANGES,
};
const COMMIT_PUSH_PAUSE: ExitAction = {
  action: "commit_push",
  label: "Commit & push",
  enabled: false,
  blocked_reason: REASON_PAUSE,
};

const PUSH_PUBLISH: ExitAction = { action: "push", label: "Push", enabled: true, publish: true };
const PUSH_COMMIT_FIRST: ExitAction = {
  action: "push",
  label: "Push",
  enabled: false,
  blocked_reason: REASON_COMMIT_FIRST,
};
const PUSH_NO_PENDING: ExitAction = {
  action: "push",
  label: "Push",
  enabled: false,
  blocked_reason: REASON_NO_PENDING,
};
const PUSH_DIVERGED: ExitAction = {
  action: "push",
  label: "Push",
  enabled: false,
  blocked_reason: REASON_DIVERGED,
};
const PUSH_PAUSE: ExitAction = {
  action: "push",
  label: "Push",
  enabled: false,
  blocked_reason: REASON_PAUSE,
};

const OPEN_PR_ENABLED: ExitAction = { action: "open_pr", label: "Open PR", enabled: true };
const OPEN_PR_COMMIT_FIRST: ExitAction = {
  action: "open_pr",
  label: "Open PR",
  enabled: false,
  blocked_reason: REASON_COMMIT_FIRST,
};
const OPEN_PR_PUSH_FIRST: ExitAction = {
  action: "open_pr",
  label: "Open PR",
  enabled: false,
  blocked_reason: REASON_PUSH_FIRST,
};
const OPEN_PR_DIVERGED: ExitAction = {
  action: "open_pr",
  label: "Open PR",
  enabled: false,
  blocked_reason: REASON_DIVERGED,
};
const OPEN_PR_NO_CHANGES: ExitAction = {
  action: "open_pr",
  label: "Open PR",
  enabled: false,
  blocked_reason: REASON_NO_PR,
};
const OPEN_PR_PAUSE: ExitAction = {
  action: "open_pr",
  label: "Open PR",
  enabled: false,
  blocked_reason: REASON_PAUSE,
};
const OPEN_BROWSER_DIRTY: ExitAction = {
  action: "open_pr",
  label: "Open in browser",
  enabled: false,
  blocked_reason: REASON_COMMIT_FIRST,
  url: COMPARE_URL,
};
const OPEN_BROWSER_CLEAN: ExitAction = {
  action: "open_pr",
  label: "Open in browser",
  enabled: true,
  url: COMPARE_URL,
};

const VIEW_PR_412: ExitAction = {
  action: "view_pr",
  label: "View PR",
  enabled: true,
  pr_number: 412,
  url: PR_412_URL,
};
const VIEW_PR_398: ExitAction = {
  action: "view_pr",
  label: "View PR",
  enabled: true,
  pr_number: 398,
  url: PR_398_URL,
};

/** Porcelain dirty total already includes the two named untracked files. */
export const DIRTY_SCOPE: WorktreeExitPlanPayload["commit_scope"] = {
  changed_files: 6,
  insertions: 142,
  deletions: 18,
  untracked_files: ["docs/notes.md", "internal/worktree/probe_test.go"],
  untracked_total: 2,
};

export const EMPTY_SCOPE: WorktreeExitPlanPayload["commit_scope"] = {
  changed_files: 0,
  insertions: 0,
  deletions: 0,
  untracked_files: [],
  untracked_total: 0,
};

export const forgeCapabilitiesFixture = {
  provider: "github",
  request_noun: "PR",
  open_action_label: "Open PR",
  view_action_label: "View PR",
  supports_draft: true,
  default_branch: "main",
  compare_url_template: "https://github.com/compozy/compozy/compare/{base}...{head}",
} as NonNullable<WorktreeExitPlanPayload["forge"]>;

function plan(overrides: Partial<WorktreeExitPlanPayload>): WorktreeExitPlanPayload {
  return {
    worktree_id: "wt_payments_retry",
    actions: [],
    commit_scope: DIRTY_SCOPE,
    cleanup: { safe: false },
    base: "main",
    ...overrides,
  } as WorktreeExitPlanPayload;
}

/** Dirty tree: Commit leads; downstream rows state the commit-first block. */
export const exitPlanCommitFixture = plan({
  primary: "commit",
  forge: forgeCapabilitiesFixture,
  actions: [COMMIT_ENABLED, COMMIT_PUSH_ENABLED, PUSH_COMMIT_FIRST, OPEN_PR_COMMIT_FIRST],
});

/** Clean tree with unpushed commits: Push leads and publishes a new branch. */
export const exitPlanPushFixture = plan({
  primary: "push",
  forge: forgeCapabilitiesFixture,
  commit_scope: EMPTY_SCOPE,
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_PUBLISH, OPEN_PR_PUSH_FIRST],
});

export const exitPlanOpenPrFixture = plan({
  primary: "open_pr",
  forge: forgeCapabilitiesFixture,
  commit_scope: EMPTY_SCOPE,
  pr_prefill: {
    title: "pedro/payments-retry",
    body: "## Summary\nDescribe the change.",
  },
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_NO_PENDING, OPEN_PR_ENABLED],
});

export const exitPlanViewPrFixture = plan({
  primary: "view_pr",
  forge: forgeCapabilitiesFixture,
  commit_scope: EMPTY_SCOPE,
  forge_status: {
    provider: "github",
    pr_number: 412,
    pr_state: "open",
    pr_url: PR_412_URL,
    merged: false,
    fetched_at: "2026-08-12T10:00:00Z",
  },
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_NO_PENDING, VIEW_PR_412],
});

/** A bound session is running: one cause blocks every row. */
export const exitPlanPausedFixture = plan({
  primary: "commit",
  forge: forgeCapabilitiesFixture,
  global_pause_cause: REASON_PAUSE,
  actions: [COMMIT_PAUSE, COMMIT_PUSH_PAUSE, PUSH_PAUSE, OPEN_PR_PAUSE],
});

export const exitPlanStatusUnreadableFixture = plan({
  commit_scope: EMPTY_SCOPE,
  global_pause_cause: REASON_UNREADABLE,
  cleanup: { safe: false, blocker: REASON_UNREADABLE },
  actions: [COMMIT_UNREADABLE],
});

export const exitPlanDivergedFixture = plan({
  forge: forgeCapabilitiesFixture,
  commit_scope: EMPTY_SCOPE,
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_DIVERGED, OPEN_PR_DIVERGED],
});

/** No remote configured: only Commit exists. */
export const exitPlanNoRemoteFixture = plan({
  primary: "commit",
  actions: [COMMIT_ENABLED],
});

/** Zero credential, dirty: compare URL exists, but open_pr stays commit-first. */
export const exitPlanZeroCredentialFixture = plan({
  primary: "commit",
  browser_url: COMPARE_URL,
  actions: [COMMIT_ENABLED, COMMIT_PUSH_ENABLED, PUSH_COMMIT_FIRST, OPEN_BROWSER_DIRTY],
});

/** Zero credential, clean: the compare link is the reachable request step. */
export const exitPlanZeroCredentialCleanFixture = plan({
  primary: "open_pr",
  commit_scope: EMPTY_SCOPE,
  browser_url: COMPARE_URL,
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_NO_PENDING, OPEN_BROWSER_CLEAN],
});

export const exitPlanMergedFixture = plan({
  primary: "view_pr",
  forge: forgeCapabilitiesFixture,
  commit_scope: EMPTY_SCOPE,
  forge_status: {
    provider: "github",
    pr_number: 398,
    pr_state: "merged",
    pr_url: PR_398_URL,
    merged: true,
    fetched_at: "2026-08-12T09:00:00Z",
  },
  cleanup: {
    safe: true,
    forge_state: "merged",
    source: "forge",
    summary: "Merged on github",
  },
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_NO_PENDING, VIEW_PR_398],
});

export const exitPlanCleanupBlockedFixture = plan({
  primary: "commit",
  cleanup: {
    safe: false,
    source: "local",
    blocker: "3 commits exist nowhere else.",
  },
  actions: [COMMIT_ENABLED],
});

/**
 * The detail hero: the agent finished its turn and is waiting on the operator,
 * which is exactly the state where exit actions unlock.
 */
export const exitPlanAwaitingInputFixture = exitPlanCommitFixture;

/** Nothing staged: the unpublished-push plan, whose commit rows carry the daemon reason. */
export const exitPlanNothingToCommitFixture = exitPlanPushFixture;

/** Daemon-resolved starting text. The web never composes or reads a template. */
export const prPrefillFixture: NonNullable<WorktreeExitPlanPayload["pr_prefill"]> = {
  title: "pedro/payments-retry",
  body: "## Summary\nDescribe the change.",
};

/** More than one template at the same stem: title is still the branch; body is empty. */
export const forgeAmbiguousTemplateFixture = {
  ...forgeCapabilitiesFixture,
  template_paths: [".github/pull_request_template.md", ".github/pull_request_template.txt"],
} as NonNullable<WorktreeExitPlanPayload["forge"]>;

export const exitPlanTemplateAmbiguousFixture = plan({
  primary: "open_pr",
  forge: forgeAmbiguousTemplateFixture,
  commit_scope: EMPTY_SCOPE,
  pr_prefill: { title: "pedro/payments-retry" },
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_NO_PENDING, OPEN_PR_ENABLED],
});

/** The PR step exists but cannot run: the daemon's reason is the whole state. */
export const exitPlanPrBlockedFixture = plan({
  forge: forgeCapabilitiesFixture,
  commit_scope: EMPTY_SCOPE,
  pr_prefill: prPrefillFixture,
  actions: [COMMIT_NO_CHANGES, COMMIT_PUSH_NO_CHANGES, PUSH_NO_PENDING, OPEN_PR_NO_CHANGES],
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

export const worktreeStatusDivergedFixture: WorktreeStatusPayload = {
  worktree_id: "wt_payments_retry",
  status: {
    branch: "pedro/payments-retry",
    detached: false,
    head_sha: "9ec3d15a2f",
    dirty_files: 0,
    insertions: 0,
    deletions: 0,
    has_upstream: true,
    ahead: 2,
    ahead_of_base: 2,
    behind: 1,
    refreshed_at: "2026-08-12T10:05:00Z",
  },
  forge: null,
} as WorktreeStatusPayload;

/** Persist-error fields only: unknowns stay null, never inferred. */
export const worktreeStatusUnknownFixture: WorktreeStatusPayload = {
  worktree_id: "wt_bench_harness",
  status: {
    branch: null,
    detached: null,
    head_sha: null,
    dirty_files: null,
    insertions: null,
    deletions: null,
    has_upstream: null,
    ahead: null,
    ahead_of_base: null,
    behind: null,
    read_error: "fatal: cannot read index",
    refreshed_at: "2026-08-12T10:05:00Z",
  },
  forge: null,
} as WorktreeStatusPayload;
