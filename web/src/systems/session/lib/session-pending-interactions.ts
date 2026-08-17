/**
 * Reads over the sanitized pending-interaction projection the daemon embeds on
 * every session payload. These are the only source of a needs-you *reason* on
 * any surface — the bell row, the toast body, and the sidebar precedence note
 * all quote `title`, never invented copy.
 *
 * A session can hold several pending questions and permissions at once, so the
 * badge derives from counts (`waiting-for-auth` outranks `waiting-for-input`)
 * and the masked side stays visible as a note rather than disappearing.
 */
import type { SessionPayload, SessionPendingInteraction } from "../types";

const PENDING_STATUSES: ReadonlySet<string> = new Set(["pending", "orphaned"]);

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
