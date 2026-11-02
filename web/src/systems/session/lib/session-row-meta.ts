// The session row metadata and accessible description.

import { formatAbsoluteTime } from "@compozy/ui";

import type { sessionBadgeSignal } from "./session-badge";
import { sessionLaterDetails, sessionLaterSpoken } from "./session-later";
import { maskedAttentionNote } from "./session-pending-interactions";
import { sessionAttentionReason, type SessionAttentionReason } from "./session-attention-reason";
import type { SessionPayload } from "../types";
import { SUBAGENT_OF_DELETED_SESSION } from "./session-subagent-summary";

export interface RowMeta {
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
  agentLabel,
  runtime,
  signal,
  orphanedSubagent,
}: {
  /** What the row calls the agent; `agent_name` stays its id. */
  agentLabel?: string;
  session: SessionPayload;
  runtime: { label: string } | null;
  signal: ReturnType<typeof sessionBadgeSignal>;
  orphanedSubagent: boolean;
}): RowMeta {
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
  const later = sessionLaterSpoken(sessionLaterDetails(session), formatAbsoluteTime);
  const spoken = [
    runtime?.label,
    [agentLabel ?? session.agent_name, ...attention, ...notes].join(" · "),
    later ? `· ${later}` : null,
  ]
    .filter(Boolean)
    .join(" ");
  return {
    visible: true,
    attention,
    reason,
    notes,
    spoken,
  };
}
