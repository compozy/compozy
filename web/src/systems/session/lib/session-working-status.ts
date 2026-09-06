// The one status line under the transcript (S3 / ADR-006 rule 3 / US-027):
// "Thinking…" until the first content, then "Working for {elapsed} · {activity}"
// from the daemon's durable turn start, still "working" while spawned agents run,
// and after the turn a frozen sentence — "Stopped by you after {d}", "Failed
// after {d} · {cause}" — or nothing when it completed (the fold carries "Worked
// for"). Every duration derives from daemon timestamps, never a browser
// stopwatch, so a remount or reconnect recomputes instead of resetting.

import type { SessionPayload } from "../types";
import { formatQuietDurationWords } from "./session-quiet-warning";
import { liveToolLabel } from "./session-tool-visual-state";

/** Live "Working for Xs" ladder: seconds under a minute, then `Xm Ys`, then `Xh Ym`. */
export function formatWorkingElapsed(startedAtMs: number, nowMs: number): string {
  const totalSeconds = Math.max(0, Math.floor((nowMs - startedAtMs) / 1000));
  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
}

/** Frozen durations read at least one second so a stop never reads "0s". */
export function formatFrozenDuration(durationMs: number): string {
  return formatWorkingElapsed(0, Math.max(1_000, durationMs));
}

/** How the last settled turn ended, as the transcript records it. */
export type SessionLastTurnCause = "completed" | "stopped" | "steer_fallback" | "failed";

/**
 * Who or what stopped the turn, from the daemon's own records — never a guess.
 * `user` is the only attribution that reads "by you"; an inactivity stop or an
 * escalated close is the daemon's doing and says so.
 */
export type SessionStopAttribution =
  | { kind: "user" }
  /** The agent did not answer the stop; the daemon escalated and verified the close (session-scope). */
  | { kind: "escalated" }
  /** Supervision stopped the session after `noWorkMs` without a work signal (`null` when the episode's start is unknown). */
  | { kind: "inactivity"; noWorkMs: number | null }
  /** Any other daemon stop (shutdown, timeout, hook…), with its stated detail when there is one. */
  | { kind: "other"; detail: string | null };

export interface SessionLastTurn {
  startedAtMs: number | null;
  endedAtMs: number | null;
  cause: SessionLastTurnCause;
  /** Short cause words for a failed turn ("rate limited"); `null` when the daemon named none. */
  failureCause: string | null;
  /** Present when `cause` is `stopped`. */
  stop?: SessionStopAttribution;
}

/** "no work for 40 minutes" — the supervision episode from its start to the stop, in words. */
export function formatNoWorkDuration(ms: number): string {
  return formatQuietDurationWords(ms);
}

/** The words after "Stopped after {d} ·" for a stop that was not the operator's; `null` for a plain stop. */
export function stopAttributionDetail(stop: SessionStopAttribution): string | null {
  switch (stop.kind) {
    case "user":
      return null;
    case "escalated":
      return "the agent didn't answer the stop, so it was closed for you";
    case "inactivity":
      return stop.noWorkMs === null
        ? "no work signal from the agent"
        : `no work for ${formatNoWorkDuration(stop.noWorkMs)}`;
    case "other":
      return stop.detail;
  }
}

export interface SessionWorkingStatusInput {
  session: Pick<SessionPayload, "activity" | "supervision" | "pending_interactions">;
  /** The turn is in flight (daemon activity or thread streaming). */
  running: boolean;
  /** No content has arrived for the in-flight turn yet. */
  thinking: boolean;
  /** The most recent settled turn; `null` when none or unknown. */
  lastTurn: SessionLastTurn | null;
  nowMs: number;
}

export type SessionWorkingStatus =
  | { kind: "hidden" }
  | { kind: "thinking" }
  | {
      kind: "working";
      /** Durable turn start (`activity.turn_started_at`); `null` shows the verb without a timer. */
      startedAtMs: number | null;
      elapsed: string | null;
      /** "Running shell", "Running 3 tools", "Waiting for your decision", or `null` between tools. */
      activity: string | null;
      /** Live children of this turn; rendered as "N agents running". */
      agentCount: number;
    }
  | {
      kind: "stopped";
      duration: string;
      /** True only for the operator's own stop: "Stopped by you after". */
      byYou: boolean;
      detail: string | null;
    }
  | { kind: "failed"; duration: string; cause: string | null };

function parseInstant(value: string | null | undefined): number | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function signalCount(session: SessionWorkingStatusInput["session"], kind: string): number {
  return session.supervision?.work_signals.filter(signal => signal.kind === kind).length ?? 0;
}

// The activity segment, in precedence: a decision waiting on the operator is
// activity too; then the honest parallel count; then the single tool's verb.
function currentActivity(session: SessionWorkingStatusInput["session"]): string | null {
  if (session.pending_interactions.length > 0) return "Waiting for your decision";
  const runningTools = signalCount(session, "tool_running");
  if (runningTools > 1) return `Running ${runningTools} tools`;
  const tool = session.activity?.current_tool?.trim();
  if (!tool) return null;
  return liveToolLabel(tool).verb;
}

export function agentCountLabel(count: number): string {
  return `${count} ${count === 1 ? "agent" : "agents"} running`;
}

export function deriveWorkingStatus(input: SessionWorkingStatusInput): SessionWorkingStatus {
  const { session, running, thinking, lastTurn, nowMs } = input;
  if (running) {
    if (thinking) return { kind: "thinking" };
    const startedAtMs = parseInstant(session.activity?.turn_started_at);
    return {
      kind: "working",
      startedAtMs,
      elapsed: startedAtMs === null ? null : formatWorkingElapsed(startedAtMs, nowMs),
      activity: currentActivity(session),
      agentCount: signalCount(session, "active_child"),
    };
  }
  if (!lastTurn || lastTurn.startedAtMs === null || lastTurn.endedAtMs === null) {
    return { kind: "hidden" };
  }
  const duration = formatFrozenDuration(lastTurn.endedAtMs - lastTurn.startedAtMs);
  switch (lastTurn.cause) {
    case "stopped": {
      const stop = lastTurn.stop ?? { kind: "user" as const };
      return {
        kind: "stopped",
        duration,
        byYou: stop.kind === "user",
        detail: stopAttributionDetail(stop),
      };
    }
    case "failed":
      return { kind: "failed", duration, cause: lastTurn.failureCause };
    default:
      // A completed or steer-replaced turn reads nothing here: the frozen
      // "Worked for" / "Interrupted after" moved to the fold in the transcript.
      return { kind: "hidden" };
  }
}
