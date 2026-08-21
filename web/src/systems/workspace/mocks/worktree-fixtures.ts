import { storyWorkspaceIds, storyWorkspaceNames } from "@/storybook/fintech-scenario";

import type {
  DiscoveredWorktreePayload,
  WorktreePayload,
  WorktreesResponse,
} from "@/systems/workspace/types";

const WORKSPACE_ID = storyWorkspaceIds.hq;
const WORKSPACE_NAME = storyWorkspaceNames.hq;

/**
 * Canonical placement (ADR-005): `<worktrees root>/<workspace>/<name>`. The
 * in-repo and sibling forms are rejected alternatives and never appear here.
 */
function worktreePath(name: string): string {
  return `/Users/ada/.compozy/worktrees/${WORKSPACE_NAME}/${name}`;
}

/** Deep enough to overflow compact dialogs and nest rows (ADR-005 form). */
export const LONG_WORKTREE_PATH =
  "/Users/ada/.compozy/worktrees/launch-hq/payments-retry-across-a-deep-operator-home-directory";

export function buildWorktreeFixture(overrides: Partial<WorktreePayload> = {}): WorktreePayload {
  const name = overrides.name ?? "payments-retry";
  return {
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    profile_archived: false,
    id: `wt_${name.replace(/-/g, "_")}`,
    workspace_id: WORKSPACE_ID,
    name,
    branch: name,
    path: worktreePath(name),
    state: "ready",
    origin: "manual",
    setup_state: "ok",
    created_branch: true,
    base_ref: "main",
    agent_activity: "idle",
    dirty: false,
    ahead: 0,
    behind: 0,
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-17T17:30:00Z",
    ...overrides,
  };
}

/** Ready, dirty, and running: the busiest realistic row. */
export const worktreeReadyDirtyRunningFixture = buildWorktreeFixture({
  name: "payments-retry",
  dirty: true,
  ahead: 3,
  agent_activity: "running",
  updated_at: "2026-04-17T17:26:00Z",
});

/** Behind its upstream with an open PR — action needed, nothing wrong. */
export const worktreeBehindFixture = buildWorktreeFixture({
  name: "auth-refresh",
  ahead: 0,
  behind: 2,
  updated_at: "2026-04-17T16:30:00Z",
});

/** Run-created, merged, safe to clean up. */
export const worktreeMergedFixture = buildWorktreeFixture({
  name: "fix-flaky-e2e",
  branch: "run/fix-flaky-e2e",
  origin: "per_run",
  run_id: "run_004",
  run_namespace: "run/",
  updated_at: "2026-04-15T09:00:00Z",
});

/** Usable but flagged: bootstrap failed and the worktree still works. */
export const worktreeSetupFlaggedFixture = buildWorktreeFixture({
  name: "data-migration",
  ahead: 1,
  setup_state: "failed",
  setup_error: "bun install exited 1",
  updated_at: "2026-04-17T14:30:00Z",
});

/** Materializing — never rendered as ready. */
export const worktreePendingFixture = buildWorktreeFixture({
  name: "docs-refresh",
  state: "pending",
  pending_phase: "branch",
  setup_state: "none",
  dirty: null,
  ahead: null,
  behind: null,
  updated_at: "2026-04-17T17:29:00Z",
});

/** Directory removed out of band; the record and its history survive. */
export const worktreeMissingFixture = buildWorktreeFixture({
  name: "hotfix-cors",
  state: "missing",
  dirty: null,
  ahead: null,
  behind: null,
  updated_at: "2026-04-15T11:00:00Z",
});

/** Status read failed — blocks dependent actions, does not mean dirty. */
export const worktreeErrorFixture = buildWorktreeFixture({
  name: "bench-harness",
  state: "error",
  dirty: null,
  ahead: null,
  behind: null,
  updated_at: "2026-04-12T10:00:00Z",
});

/** Creation rolled back. Distinct from a status read failure (N-004). */
export const worktreeFailedFixture = buildWorktreeFixture({
  name: "spike-rollback",
  state: "failed",
  dirty: null,
  ahead: null,
  behind: null,
});

/**
 * External checkout with no record. Adopted externals keep their foreign path,
 * so this one stays outside the canonical root before and after adoption.
 */
export const discoveredWorktreeFixture: DiscoveredWorktreePayload = {
  name: "spike-sqlite-vacuum",
  branch: "spike/sqlite-vacuum",
  path: "/Users/ada/dev/northstar-spike",
  detached: false,
  selectable: true,
  stale: false,
  unavailable: false,
};

/** Prunable: git knows it, but it cannot be adopted. */
export const discoveredStaleWorktreeFixture: DiscoveredWorktreePayload = {
  name: "abandoned-probe",
  branch: "spike/abandoned",
  path: "/Users/ada/dev/abandoned-probe",
  detached: false,
  selectable: false,
  stale: true,
  unavailable: true,
  reason: "prunable",
};

/** Broken checkout carrying git's own long reason — the capped reason lane case. */
export const discoveredBrokenWorktreeFixture: DiscoveredWorktreePayload = {
  name: "imersao-aovivo",
  branch: "main",
  path: "/Users/ada/Desktop/imersao-aovivo",
  detached: false,
  selectable: false,
  stale: false,
  unavailable: true,
  reason: "gitdir file points to non-existent location",
};

export const worktreeFixtures: WorktreePayload[] = [
  worktreeReadyDirtyRunningFixture,
  worktreeBehindFixture,
  worktreeMergedFixture,
  worktreeSetupFlaggedFixture,
  worktreePendingFixture,
  worktreeMissingFixture,
  worktreeErrorFixture,
];

const GIT_BACKED_REPO: WorktreesResponse["repo"] = { git_backed: true, git_available: true };

export const worktreeListingFixture: WorktreesResponse = {
  worktrees: worktreeFixtures,
  discovered: [discoveredWorktreeFixture],
  repo: GIT_BACKED_REPO,
};

/** Git-backed with nothing created yet: no group noise, only the create path. */
export const emptyWorktreeListingFixture: WorktreesResponse = {
  worktrees: [],
  discovered: [],
  repo: GIT_BACKED_REPO,
};

/** Not a git repository: every worktree affordance is absent, never disabled. */
export const nonGitWorktreeListingFixture: WorktreesResponse = {
  worktrees: [],
  discovered: [],
  repo: { git_backed: false, git_available: true },
};

/** First scan of a repo where nothing has been adopted yet. */
export const discoveredOnlyWorktreeListingFixture: WorktreesResponse = {
  worktrees: [],
  discovered: [discoveredWorktreeFixture, discoveredStaleWorktreeFixture],
  repo: GIT_BACKED_REPO,
};

/** Past the truncation limit, so the nest folds behind an overflow row. */
export const manyWorktreesListingFixture: WorktreesResponse = {
  worktrees: [
    ...worktreeFixtures,
    buildWorktreeFixture({ name: "ledger-export", updated_at: "2026-04-16T09:00:00Z" }),
    buildWorktreeFixture({ name: "webhook-retry", updated_at: "2026-04-16T08:00:00Z" }),
  ],
  discovered: [discoveredWorktreeFixture],
  repo: GIT_BACKED_REPO,
};

export const scrollableWorktreesListingFixture: WorktreesResponse = {
  worktrees: Array.from({ length: 18 }, (_, index) => {
    const suffix = String(index + 1).padStart(2, "0");
    return buildWorktreeFixture({
      name: `detached-${suffix}`,
      branch: `feature/detached-${suffix}`,
      updated_at: `2026-04-${suffix}T08:00:00Z`,
    });
  }),
  discovered: [],
  repo: GIT_BACKED_REPO,
};
