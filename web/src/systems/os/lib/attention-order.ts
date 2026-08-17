/**
 * Attention ordering, shared by the bell sections, the attention-first sidebar
 * sort, and the palette Sessions view so all three agree.
 *
 * The daemon stamps `attention_changed_at` inside the same transaction as the
 * transition (including the event-less `done → idle` seen-clear), so it is the
 * restart-stable ordering key. Ties break by session id, which keeps the order
 * total and stable across recomputes — a row never jitters between refreshes.
 */
import { sessionAttentionClass, toSessionBadge, type SessionPayload } from "@/systems/session";

/** Band order for the attention-first sort: needs-you, finished, working, rest. */
export const ATTENTION_BANDS = ["needs-you", "finished", "working", "rest"] as const;

export type AttentionBand = (typeof ATTENTION_BANDS)[number];

export function attentionBand(badge: string | null | undefined): AttentionBand {
  const attention = sessionAttentionClass(badge);
  if (attention === "needs-you") return "needs-you";
  if (attention === "finished") return "finished";
  return toSessionBadge(badge) === "running" ? "working" : "rest";
}

function bandRank(badge: string | null | undefined): number {
  return ATTENTION_BANDS.indexOf(attentionBand(badge));
}

/**
 * The transition timestamp a row sorts on. Sessions that never carried an
 * attention transition fall back to their last update, so an unstamped row
 * still lands in a defensible place instead of at the epoch.
 */
export function attentionOrderKey(session: SessionPayload): string {
  return session.attention_changed_at ?? session.updated_at;
}

/** Newest transition first, ties by session id. */
export function compareAttentionRecency(a: SessionPayload, b: SessionPayload): number {
  const left = attentionOrderKey(a);
  const right = attentionOrderKey(b);
  if (left !== right) return left < right ? 1 : -1;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}

/** Attention-first: band, then newest transition, then session id. */
export function compareAttentionFirst(a: SessionPayload, b: SessionPayload): number {
  const bands = bandRank(a.badge) - bandRank(b.badge);
  if (bands !== 0) return bands;
  return compareAttentionRecency(a, b);
}

export function sortAttentionFirst(sessions: readonly SessionPayload[]): SessionPayload[] {
  return [...sessions].sort(compareAttentionFirst);
}
