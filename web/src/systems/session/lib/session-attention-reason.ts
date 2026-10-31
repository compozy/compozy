// Why a session reads "Needs attention" (#760). The daemon raises the badge for
// a stop it could not verify (`attention`), for a child whose turn ended without
// delivering (`attention: "ended_early"`, #1065), or for supervision evidence it could
// not trust (`internal/session/work_signal_registry.go`): a source that reported
// an explicit blocker (`attention`), went stale (`stale`), or could not be read
// (`unknown`). Both causes already ride on the session payload, so the reason
// is read here, never inferred.

import type { SessionPayload, SessionWorkSignalKind } from "../types";
import { sessionStopAttention } from "./session-stop-attention";

type AttentionSource = Pick<SessionPayload, "attention" | "state" | "supervision">;

export interface SessionAttentionReason {
  /** One short phrase for the row's secondary line. */
  summary: string;
  /** Every cause, one per line, for the hover title. */
  detail: string;
}

const SIGNAL_NOUN: Record<SessionWorkSignalKind, string> = {
  agent_progress: "agent progress",
  tool_running: "running tool",
  active_child: "subagent",
  loop_run: "Loop run",
  task_lease: "task lease",
  scheduled_wait: "scheduled wait",
  active_monitor: "monitor",
  awaiting_reply: "awaited reply",
};

const STOP_REASON = "Stop not confirmed";

/** The daemon's attention code for a child turn that ended without delivering. */
export const ENDED_EARLY_ATTENTION = "ended_early";

const ENDED_EARLY_REASON = "Stopped before delivering";
const ENDED_EARLY_DETAIL =
  "Stopped before delivering: no pull request, and nothing left running to resume it";

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function signalNoun(kind: string): string {
  return SIGNAL_NOUN[kind as SessionWorkSignalKind] ?? kind.replaceAll("_", " ");
}

type SourceState = NonNullable<SessionPayload["supervision"]>["sources"][number];

function sourceSummary(source: SourceState): string {
  const noun = signalNoun(source.kind);
  if (source.state === "attention") return source.error?.trim() || `${capitalize(noun)} is blocked`;
  if (source.state === "stale") return `${capitalize(noun)} went stale`;
  return `Can't check ${noun}`;
}

function sourceDetail(source: SourceState): string {
  const summary = sourceSummary(source);
  const error = source.error?.trim();
  return source.state !== "attention" && error ? `${summary}: ${error}` : summary;
}

// Explicit blockers first, then stale evidence, then sources that could not be read.
const SOURCE_RANK: Record<string, number> = { attention: 0, stale: 1, unknown: 2 };

/**
 * The cause behind a `needs-attention` badge, or `null` for every other badge
 * and for a payload that names no cause.
 */
export function sessionAttentionReason(
  session: AttentionSource,
  badge: string
): SessionAttentionReason | null {
  if (badge !== "needs-attention") return null;
  const lines: string[] = [];
  const summaries: string[] = [];
  if (sessionStopAttention(session)) {
    lines.push(STOP_REASON);
    summaries.push(STOP_REASON);
  } else if (session.attention === ENDED_EARLY_ATTENTION) {
    lines.push(ENDED_EARLY_DETAIL);
    summaries.push(ENDED_EARLY_REASON);
  }
  const sources = (session.supervision?.sources ?? [])
    .filter(source => source.state in SOURCE_RANK)
    .sort((a, b) => SOURCE_RANK[a.state]! - SOURCE_RANK[b.state]!);
  for (const source of sources) {
    lines.push(sourceDetail(source));
    summaries.push(sourceSummary(source));
  }
  if (summaries.length === 0) return null;
  const more = summaries.length - 1;
  return {
    summary: more > 0 ? `${summaries[0]} (+${more})` : summaries[0]!,
    detail: lines.join("\n"),
  };
}
