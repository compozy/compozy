/**
 * Reads over the sanitized pending-interaction projection the daemon embeds on
 * every session payload. These are the only source of a needs-you *reason* on
 * any surface — the bell row, the toast body, and the sidebar precedence note
 * all quote that projection. Terminal tool-id titles are rewritten to the
 * board's verb; every other title stays as published.
 *
 * A session can hold several pending questions and permissions at once, so the
 * badge derives from counts (`waiting-for-auth` outranks `waiting-for-input`)
 * and the masked side stays visible as a note rather than disappearing.
 */
import { terminalAttentionReason } from "@/systems/terminal/parts";

import type { SessionInteractionRecord, SessionPayload, SessionPendingInteraction } from "../types";

const PENDING_STATUSES: ReadonlySet<string> = new Set(["pending", "orphaned"]);

/** Daemon resolution written when boot reconciliation expires a pre-crash decision. */
export const RESTART_EXPIRED_RESOLUTION = "failed-by-restart";

export function pendingInteractions(session: SessionPayload): SessionPendingInteraction[] {
  return session.pending_interactions.filter(interaction =>
    PENDING_STATUSES.has(interaction.status)
  );
}

function countOfKind(session: SessionPayload, kind: string): number {
  return pendingInteractions(session).filter(interaction => interaction.kind === kind).length;
}

export function pendingClarifyCount(session: SessionPayload): number {
  return countOfKind(session, "clarify");
}

export function pendingPermissionCount(session: SessionPayload): number {
  return countOfKind(session, "permission");
}

/**
 * The reason line for a needs-you session: the newest pending interaction's
 * sanitized title. Returns null when the daemon reports no pending content, so
 * callers fall back to the state word instead of guessing.
 */
export function pendingInteractionReason(session: SessionPayload): string | null {
  const rows = pendingInteractions(session);
  if (rows.length === 0) return null;
  const preferred =
    rows.find(interaction => interaction.kind === "permission") ?? rows[rows.length - 1] ?? rows[0];
  const rewritten = terminalAttentionReason(preferred?.title, preferred?.tool_id);
  if (rewritten) return rewritten;
  const title = preferred?.title?.trim();
  return title ? title : null;
}

/**
 * The masked half of a precedence collision (US-001.EC-1): a permission gate
 * hides a pending question on the row, so the row names how many are waiting
 * behind it. Null when nothing is masked.
 */
export function maskedAttentionNote(session: SessionPayload, badge: string): string | null {
  if (badge !== "waiting-for-auth") return null;
  const questions = pendingClarifyCount(session);
  if (questions === 0) return null;
  return questions === 1 ? "+1 question" : `+${questions} questions`;
}

/**
 * Settled decisions the transcript never recorded an answer for, keyed by the provider
 * request id the transcript's permission/clarify parts carry. Only `canceled` rows
 * qualify: a resolved or timed-out decision reaches the transcript as its own event, so
 * projecting it here would double the receipt.
 */
export function expiredInteractionsByRequest(
  rows: readonly SessionInteractionRecord[]
): ReadonlyMap<string, SessionInteractionRecord> {
  const byRequest = new Map<string, SessionInteractionRecord>();
  for (const row of rows) {
    if (row.status !== "canceled") continue;
    const requestId = row.provider_request_id.trim();
    if (requestId) byRequest.set(requestId, row);
  }
  return byRequest;
}

/** The daemon expired this decision because it restarted before anyone answered. */
export function interactionExpiredByRestart(row: SessionInteractionRecord): boolean {
  return row.status === "canceled" && row.resolution === RESTART_EXPIRED_RESOLUTION;
}

/**
 * Permission decisions the daemon applied, keyed by the provider request id the
 * transcript's permission parts carry. The transcript part records the decision but
 * never who made it; the `resolved` row's `resolved_by` is the only attribution
 * evidence. The daemon's uniqueness is (session, kind, provider request id), so a
 * resolved clarification may share the id: only `permission` rows attribute a receipt.
 */
export function resolvedInteractionsByRequest(
  rows: readonly SessionInteractionRecord[]
): ReadonlyMap<string, SessionInteractionRecord> {
  const byRequest = new Map<string, SessionInteractionRecord>();
  for (const row of rows) {
    if (row.kind !== "permission" || row.status !== "resolved") continue;
    const requestId = row.provider_request_id.trim();
    if (requestId) byRequest.set(requestId, row);
  }
  return byRequest;
}

/**
 * Who settled a permission ask, read from the daemon's `resolved_by` actor:
 * `you` for an operator surface (`operator`, `operator:control`), `agent` for another
 * session's native approval (`agent_session:<id>`), `timeout` when the ask expired
 * unanswered, `runtime` when the row names the runtime (`provider`, `system` — also the
 * daemon's fallback actor, so it never proves whether anyone was asked), and `unknown`
 * when no row or an unrecognized actor leaves no evidence — never a guessed person.
 */
export type PermissionDecisionActor = "you" | "agent" | "timeout" | "runtime" | "unknown";

export function permissionDecisionActor(
  row: SessionInteractionRecord | undefined
): PermissionDecisionActor {
  const resolvedBy = row?.status === "resolved" ? (row.resolved_by?.trim() ?? "") : "";
  if (resolvedBy === "operator" || resolvedBy.startsWith("operator:")) return "you";
  if (resolvedBy.startsWith("agent_session:")) return "agent";
  if (resolvedBy === "timeout") return "timeout";
  if (resolvedBy === "provider" || resolvedBy === "system") return "runtime";
  return "unknown";
}
