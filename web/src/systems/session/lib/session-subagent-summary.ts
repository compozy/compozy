import type { SessionPayload } from "../types";

/** COPY.md: the lineage label once a subagent's parent session is gone. */
export const SUBAGENT_OF_DELETED_SESSION = "Subagent of a deleted session";

/** Chip counts, structurally the subagent components' `SubagentCounts`. */
export interface SessionSubagentCounts {
  live: number;
  total: number;
  failed: number;
  attention: number;
}

/**
 * The parent row's `subagent_summary` as chip counts; `null` when it has none.
 *
 * `most_urgent` is deliberately not read: the daemon derives it from these
 * same counts in the same row with the chip's precedence (attention > failed >
 * running), so it adds nothing; the chip still needs the parent's own turn to
 * turn `running` into `delegated`, and the stream-fed roster yields counts
 * (`subagentCounts`) but no `most_urgent`, so counts stay the one chip input.
 */
export function sessionSubagentCounts(session: SessionPayload): SessionSubagentCounts | null {
  const summary = session.subagent_summary;
  if (!summary || summary.total <= 0) return null;
  return {
    live: summary.live,
    total: summary.total,
    failed: summary.failed,
    attention: summary.attention,
  };
}
