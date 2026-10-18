// The session row's meta line model: what the line says, what a child inherits
// from its thread root, and the words that name the row to assistive tech.

import type { ProfileOwner } from "@/systems/profiles";

import type { sessionBadgeSignal } from "./session-badge";
import { maskedAttentionNote } from "./session-pending-interactions";
import { sessionAttentionReason, type SessionAttentionReason } from "./session-attention-reason";
import type { SessionPayload } from "../types";
import { SUBAGENT_OF_DELETED_SESSION } from "./session-subagent-summary";

export interface RowMeta {
  /** Same agent, provider, and model as the thread root: the child drops its who line. */
  inherits: boolean;
  /** The meta line has something of its own to show. */
  visible: boolean;
  /** Attention words that never truncate. */
  attention: string[];
  /** Why the daemon raised attention; its detail is the row's hover title. */
  reason: SessionAttentionReason | null;
  /** Secondary notes, the first to give way when the line is full. */
  notes: string[];
  /** The row's accessible description, in reading order. */
  spoken: string;
}

export function rowMeta({
  session,
  parent,
  owner,
  runtime,
  signal,
  orphanedSubagent,
}: {
  session: SessionPayload;
  parent: SessionPayload | undefined;
  owner: ProfileOwner | undefined;
  runtime: { label: string } | null;
  signal: ReturnType<typeof sessionBadgeSignal>;
  orphanedSubagent: boolean;
}): RowMeta {
  const inherits =
    parent !== undefined &&
    owner === undefined &&
    parent.agent_name === session.agent_name &&
    (parent.runtime?.effective?.provider ?? null) ===
      (session.runtime?.effective?.provider ?? null) &&
    (parent.runtime?.effective?.model ?? null) === (session.runtime?.effective?.model ?? null);
  const attention: string[] = [];
  // The mark carries the state; the word speaks up whenever the glyph is shared.
  if (signal.speaks) attention.push(signal.displayLabel);
  const reason = sessionAttentionReason(session, signal.label);
  if (reason !== null) attention.push(reason.summary);
  const masked = maskedAttentionNote(session, signal.label);
  if (masked !== null) attention.push(masked);
  const notes: string[] = [];
  if (orphanedSubagent) notes.push(SUBAGENT_OF_DELETED_SESSION);
  if (session.archived_at !== null) notes.push("Archived");
  const spoken = [runtime?.label, [session.agent_name, ...attention, ...notes].join(" · ")]
    .filter(Boolean)
    .join(" ");
  return {
    inherits,
    visible: !inherits || attention.length > 0 || notes.length > 0,
    attention,
    reason,
    notes,
    spoken,
  };
}
