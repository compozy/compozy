/**
 * The dock's terminal badge.
 *
 * Counts what is waiting on this person in this project, under the profile they
 * are working as. The projection is built here and registered by the activation
 * tranche; it carries its scope key so a switch cannot leave the previous
 * profile's count on screen while the new scope loads.
 */

import type { TerminalInfo, TerminalInputRequest } from "../types";

/** A terminal approval still waiting for a decision. */
export interface TerminalPendingApproval {
  terminalId: string;
  profileId: string;
}

export interface TerminalBadgeInput {
  /** `(workspace, profile)` identity these rows were read under. */
  scopeKey: string;
  profileId: string;
  /** Only the owning profile is read, so any row carrying one qualifies. */
  inputRequests: readonly Pick<TerminalInputRequest, "profile_id">[];
  pendingApprovals: readonly TerminalPendingApproval[];
}

export interface TerminalBadgeProjection {
  scopeKey: string;
  /** Undefined when nothing is waiting: zero renders nothing at all. */
  count: number | undefined;
}

/**
 * Questions and approvals waiting on you, for this profile in this project.
 *
 * Rows are filtered by owning profile even though the server already scoped
 * them: a live merge that arrived a moment before a switch must not survive it.
 */
export function projectTerminalBadge(input: TerminalBadgeInput): TerminalBadgeProjection {
  const requests = input.inputRequests.filter(
    request => request.profile_id === input.profileId
  ).length;
  const approvals = input.pendingApprovals.filter(
    approval => approval.profileId === input.profileId
  ).length;
  const total = requests + approvals;
  return { scopeKey: input.scopeKey, count: total === 0 ? undefined : total };
}

/** Whether any terminal in scope is alive — the dock's running dot. */
export function terminalsRunning(terminals: readonly Pick<TerminalInfo, "state">[]): boolean {
  return terminals.some(terminal => terminal.state === "running");
}
