import type {
  WorktreeExitAction,
  WorktreeExitCleanupEvidence,
  WorktreeExitCommitScope,
  WorktreeExitPlanPayload,
  WorktreeForgeCapabilities,
} from "../types";

/**
 * Actions that open a dialog before anything runs. Purely a UI routing fact —
 * whether the action is *allowed* is the daemon's call and lives in `enabled`.
 */
const ACTIONS_REQUIRING_INPUT = new Set<string>(["commit", "commit_push", "open_pr"]);
/** Actions whose whole effect is navigating to an already-existing URL. */
const ACTIONS_OPENING_URL = new Set<string>(["view_pr"]);

export interface WorktreeExitLadderRow {
  action: WorktreeExitAction;
  /** Provider vocabulary from the plan. Never a web-side default. */
  label: string;
  enabled: boolean;
  /** Payload literal, rendered verbatim on the menu row and the disabled title. */
  blockedReason?: string;
  publish: boolean;
  url?: string;
  prNumber?: number;
  isPrimary: boolean;
  requiresInput: boolean;
  opensURL: boolean;
}

export interface WorktreeExitLadder {
  worktreeID: string;
  rows: WorktreeExitLadderRow[];
  /** The daemon's computed primary; falls back to the first row so the control always names itself. */
  primary?: WorktreeExitLadderRow;
  menuRows: WorktreeExitLadderRow[];
  /** True when one cause blocks every action (status unreadable, bound session running). */
  blocked: boolean;
  /** The cause literal for that block, rendered verbatim. */
  blockedReason?: string;
  base?: string;
  browserURL?: string;
  commitScope: WorktreeExitCommitScope;
  cleanup: WorktreeExitCleanupEvidence;
  /** Absent ⇒ no forge serves this remote; PR affordances are absent, never disabled. */
  forge?: WorktreeForgeCapabilities;
  forgeStatus?: NonNullable<WorktreeExitPlanPayload["forge_status"]>;
  /** True when the plan carries an open/merged request record from the provider. */
  hasForgeStatus: boolean;
}

function toRow(
  row: WorktreeExitPlanPayload["actions"][number],
  primary: string | undefined
): WorktreeExitLadderRow {
  return {
    action: row.action,
    label: row.label,
    enabled: row.enabled,
    blockedReason: row.blocked_reason,
    publish: row.publish ?? false,
    url: row.url,
    prNumber: row.pr_number,
    isPrimary: primary !== undefined && row.action === primary,
    requiresInput: ACTIONS_REQUIRING_INPUT.has(row.action),
    opensURL: ACTIONS_OPENING_URL.has(row.action),
  };
}

/**
 * Projects the daemon's exit plan into the split-button view model.
 *
 * This is a projection, not a decision: row order, labels, enablement, and every
 * blocked reason come straight off the payload. The web adds only which rows
 * open a dialog and which row the split button shows first.
 */
export function toWorktreeExitLadder(plan: WorktreeExitPlanPayload): WorktreeExitLadder {
  const rows = plan.actions.map(row => toRow(row, plan.primary));
  const primary = rows.find(row => row.isPrimary) ?? rows[0];
  const blockedReason = plan.global_pause_cause?.trim();
  return {
    worktreeID: plan.worktree_id,
    rows,
    primary,
    menuRows: primary ? rows.filter(row => row.action !== primary.action) : rows,
    blocked: Boolean(blockedReason),
    blockedReason: blockedReason === "" ? undefined : blockedReason,
    base: plan.base,
    browserURL: plan.browser_url,
    commitScope: plan.commit_scope,
    cleanup: plan.cleanup,
    forge: plan.forge ?? undefined,
    forgeStatus: plan.forge_status ?? undefined,
    hasForgeStatus: Boolean(plan.forge_status),
  };
}

/**
 * Porcelain dirty total from the daemon. Untracked names are listed separately;
 * they are already counted in `changed_files`.
 */
export function commitScopeTotal(scope: WorktreeExitCommitScope): number {
  return scope.changed_files;
}

export function isCommitScopeEmpty(scope: WorktreeExitCommitScope): boolean {
  return commitScopeTotal(scope) === 0;
}
