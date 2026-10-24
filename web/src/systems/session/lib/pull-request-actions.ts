/**
 * The PR chip menu (`_uiux.md` S2, Business Rules 14–15): one primary next
 * action, then Open PR and Refresh, then whichever agent prompts remain.
 * Unavailable actions are absent, never disabled.
 */
import type { SessionPullRequest } from "./session-pull-request";

/** Exit-plan actions the chip may lead with; `view_pr` is Open PR's job. */
const CHIP_EXIT_ACTIONS = new Set(["commit_push", "push", "open_pr"]);

export interface PullRequestExitCandidate {
  action: string;
  /** Provider vocabulary from the plan, rendered verbatim. */
  label: string;
  enabled: boolean;
  blockedReason?: string;
}

export interface PullRequestExitPlanView {
  primary?: PullRequestExitCandidate;
  /** One cause blocks every exit action (status unreadable, bound session running). */
  blocked: boolean;
  blockedReason?: string;
}

export type PullRequestMenuItem =
  | { id: "exit"; label: string; action: string }
  | { id: "fix"; label: "Fix failing checks" }
  | { id: "address"; label: "Address review comments" }
  | { id: "open"; label: "Open PR"; url: string }
  | { id: "refresh"; label: "Refresh" };

export interface PullRequestMenuModel {
  primary: PullRequestMenuItem | null;
  /** The exit plan's reason when it blocked its own action; the primary item's tooltip. */
  primaryReason?: string;
  rest: PullRequestMenuItem[];
  prompts: PullRequestMenuItem[];
}

export interface PullRequestMenuInput {
  pullRequest: SessionPullRequest | null | undefined;
  /** User-controllable sessions get exit actions and agent prompts. */
  controllable: boolean;
  /** The bound managed worktree's exit plan; absent for workspace-root sessions. */
  exitPlan?: PullRequestExitPlanView;
}

function exitItem(input: PullRequestMenuInput): {
  item: PullRequestMenuItem | null;
  reason?: string;
} {
  const plan = input.exitPlan;
  const primary = plan?.primary;
  if (!input.controllable || !plan || !primary || !CHIP_EXIT_ACTIONS.has(primary.action)) {
    return { item: null };
  }
  if (plan.blocked) return { item: null, reason: plan.blockedReason };
  if (!primary.enabled) return { item: null, reason: primary.blockedReason };
  return { item: { id: "exit", label: primary.label, action: primary.action } };
}

export function pullRequestMenuModel(input: PullRequestMenuInput): PullRequestMenuModel {
  const pr = input.pullRequest;
  const open = pr?.state === "open";
  const canFix = input.controllable && open && pr.checks.state === "failing";
  const canAddress = input.controllable && open && pr.review_decision === "changes_requested";
  const openItem: PullRequestMenuItem | null = pr?.url
    ? { id: "open", label: "Open PR", url: pr.url }
    : null;
  const fix: PullRequestMenuItem = { id: "fix", label: "Fix failing checks" };
  const address: PullRequestMenuItem = { id: "address", label: "Address review comments" };

  const exit = exitItem(input);
  const primary = exit.item ?? (canFix ? fix : canAddress ? address : openItem);
  const rest: PullRequestMenuItem[] = [];
  if (openItem && primary !== openItem) rest.push(openItem);
  rest.push({ id: "refresh", label: "Refresh" });
  const prompts: PullRequestMenuItem[] = [];
  if (canFix && primary !== fix) prompts.push(fix);
  if (canAddress && primary !== address) prompts.push(address);

  return {
    primary,
    ...(exit.reason ? { primaryReason: exit.reason } : {}),
    rest,
    prompts,
  };
}

const MAX_FAILED_NAMES = 5;

/** One normal user prompt asking this session's agent to fix the failing checks. */
export function buildFixChecksPrompt(pr: SessionPullRequest): string {
  const names = pr.checks.failed_names.slice(0, MAX_FAILED_NAMES);
  const failing = names.length > 0 ? ` Failing checks: ${names.join(", ")}.` : "";
  return [
    `The checks on pull request #${pr.number} (${pr.url}) are failing at head ${pr.head_sha}.${failing}`,
    "Investigate each failure, fix the cause, and push the fix to this branch.",
  ].join("\n\n");
}

/** One normal user prompt asking this session's agent to address the requested changes. */
export function buildAddressReviewPrompt(pr: SessionPullRequest): string {
  return [
    `Pull request #${pr.number} (${pr.url}) has a review requesting changes at head ${pr.head_sha}.`,
    "Read the review comments, address each one, and push the changes to this branch.",
  ].join("\n\n");
}
