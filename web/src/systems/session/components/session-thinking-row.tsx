import { CircleAlert, CircleStop, LoaderCircle } from "lucide-react";

import { SessionSummaryDisclosure } from "./session-summary-disclosure";

import { TypingDots } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { useSecondClock } from "@/hooks/use-second-clock";
import { formatMessageTimestamp } from "../lib/format-timestamp";
import {
  agentCountLabel,
  deriveWorkingStatus,
  formatWorkingElapsed,
  type SessionLastTurn,
  type SessionWorkingStatus,
  type SessionWorkingStatusInput,
} from "../lib/session-working-status";

export interface SessionThinkingRowProps {
  session: SessionWorkingStatusInput["session"];
  /** The turn is in flight (daemon activity or thread streaming). */
  running: boolean;
  /** Stop requested; no terminal outcome has been confirmed yet. */
  stopping?: boolean;
  /** No content has arrived for the in-flight turn yet. */
  thinking: boolean;
  /** The most recent settled turn, from the transcript. */
  lastTurn: SessionLastTurn | null;
  /** Pauses the 1 Hz clock while the window is not live; the row then says "as of {time}". */
  liveDataEnabled?: boolean;
  /** When the window stopped updating; shown as "as of HH:MM" while paused. */
  pausedAtMs?: number | null;
  /** Reduced motion: no dots, no shimmer — the words carry the state. */
  reducedMotion?: boolean;
  /**
   * The daemon answered the operator's stop with `nothing-in-flight`: the turn
   * had already finished (US-009.EC-2). A faint, transient note; the turn
   * folds normally and nothing reads "canceled".
   */
  stopCompletionNote?: boolean;
}

const ROW_CLASS =
  "flex min-h-transcript-row min-w-0 items-center gap-2 px-1 py-0.5 text-transcript-meta text-subtle tabular-nums";

function SessionStatusSeparator() {
  return (
    <span aria-hidden="true" className="px-1.5 text-faint">
      ·
    </span>
  );
}

function WorkingTimer({
  startedAtMs,
  enabled,
  fallback,
}: {
  startedAtMs: number;
  enabled: boolean;
  fallback: string | null;
}) {
  const now = useSecondClock(enabled);
  return (
    <span data-testid="session-working-timer" className="font-medium text-muted">
      {enabled || fallback === null ? formatWorkingElapsed(startedAtMs, now) : fallback}
    </span>
  );
}

function Dots({ reducedMotion }: { reducedMotion: boolean }) {
  if (reducedMotion) return null;
  return <TypingDots className="session-working-dots gap-transcript-meta-gap [&>span]:bg-faint" />;
}

/** Keeps elapsed time and agent counts beside an inspectable activity summary. */
function WorkingStatusLine({
  status,
  liveDataEnabled,
  pausedAtMs,
  reducedMotion,
  activityDetail,
}: {
  status: Extract<SessionWorkingStatus, { kind: "working" }>;
  liveDataEnabled: boolean;
  pausedAtMs: number | null;
  reducedMotion: boolean;
  activityDetail?: string;
}) {
  const asOf = !liveDataEnabled && pausedAtMs !== null ? formatMessageTimestamp(pausedAtMs) : null;
  return (
    <div
      role="status"
      aria-label="Working"
      data-testid="session-working-row"
      data-status="working"
      data-agents={status.agentCount || undefined}
      className={ROW_CLASS}
    >
      <Dots reducedMotion={reducedMotion || !liveDataEnabled} />
      <span className="flex min-w-0 flex-1 items-center">
        <span className="shrink-0 whitespace-nowrap">
          {status.startedAtMs !== null ? (
            <>
              Working for{" "}
              <WorkingTimer
                startedAtMs={status.startedAtMs}
                enabled={liveDataEnabled}
                fallback={status.elapsed}
              />
            </>
          ) : (
            "Working…"
          )}
        </span>
        {status.activity ? (
          <>
            <SessionStatusSeparator />
            <span data-testid="session-working-activity" className="min-w-0 max-w-sm">
              <SessionSummaryDisclosure
                label="Activity details"
                summary={status.activity}
                detail={activityDetail ?? status.activity}
                className="max-w-full"
              />
            </span>
          </>
        ) : null}
        {status.agentCount > 0 ? (
          <>
            <SessionStatusSeparator />
            <span data-testid="session-working-agents" className="shrink-0 whitespace-nowrap">
              {agentCountLabel(status.agentCount)}
            </span>
          </>
        ) : null}
        {asOf ? (
          <>
            <SessionStatusSeparator />
            <span data-testid="session-working-as-of" className="shrink-0 whitespace-nowrap">
              as of {asOf}
            </span>
          </>
        ) : null}
      </span>
    </div>
  );
}

/** Renders the session status with bounded previews and complete detail access. */
function SessionStatusLine({
  status,
  liveDataEnabled,
  pausedAtMs,
  reducedMotion,
  activityDetail,
}: {
  status: SessionWorkingStatus;
  liveDataEnabled: boolean;
  pausedAtMs: number | null;
  reducedMotion: boolean;
  activityDetail?: string;
}) {
  switch (status.kind) {
    case "thinking": {
      // A paused window shows no cadence either: still text, the word carries the state.
      const still = reducedMotion || !liveDataEnabled;
      return (
        <div
          role="status"
          aria-label="Agent is responding"
          data-testid="session-thinking-row"
          data-status="thinking"
          className={ROW_CLASS}
        >
          <Dots reducedMotion={still} />
          <span className={cn("font-medium", still ? "text-subtle" : "session-shimmer")}>
            Thinking…
          </span>
        </div>
      );
    }
    case "working":
      return (
        <WorkingStatusLine
          status={status}
          liveDataEnabled={liveDataEnabled}
          pausedAtMs={pausedAtMs}
          reducedMotion={reducedMotion}
          activityDetail={activityDetail}
        />
      );
    case "stopped":
      return (
        <div
          role="status"
          data-testid="session-stopped-row"
          data-status="stopped"
          data-stopped-by={status.byYou ? "you" : "daemon"}
          className={cn(ROW_CLASS, "text-warning")}
        >
          <CircleStop aria-hidden="true" className="size-3 shrink-0" />
          <span>
            {status.byYou ? "Stopped by you after " : "Stopped after "}
            <span className="font-medium">{status.duration}</span>
            {status.detail ? (
              <>
                <SessionStatusSeparator />
                {status.detail}
              </>
            ) : null}
          </span>
        </div>
      );
    case "failed":
      return (
        <div
          role="status"
          data-testid="session-failed-row"
          data-status="failed"
          className={cn(ROW_CLASS, "text-danger")}
        >
          <CircleAlert aria-hidden="true" className="size-3 shrink-0" />
          <span>
            Failed after <span className="font-medium">{status.duration}</span>
            {status.cause ? (
              <>
                <SessionStatusSeparator />
                {status.cause}
              </>
            ) : null}
          </span>
        </div>
      );
    case "hidden":
      return null;
  }
}

/**
 * `SessionThinkingRow` (S3): the one line that says what the agent is doing
 * right now. "Thinking…" from your send until the first word or tool, then
 * "Working for 2m 14s · Running shell", still working while spawned agents
 * run ("· 2 agents running"), and after a stop a frozen "Stopped by you after
 * 1m 40s" in warning ink — the only tone it wears besides danger for a real
 * failure. A completed turn reads nothing here: its "Worked for" moved to the
 * fold. Elapsed is the daemon's, so it survives a reconnect; a paused window
 * freezes the clock and says "as of HH:MM".
 */
export function SessionThinkingRow({
  session,
  running,
  thinking,
  lastTurn,
  liveDataEnabled = true,
  pausedAtMs = null,
  reducedMotion = false,
  stopCompletionNote = false,
  stopping = false,
}: SessionThinkingRowProps) {
  const now = useSecondClock(false);
  const status = deriveWorkingStatus({ session, running, thinking, lastTurn, nowMs: now });
  if (stopping) {
    return (
      <div role="status" data-testid="session-stopping-row" className={ROW_CLASS}>
        <LoaderCircle
          aria-hidden="true"
          className={cn("size-3", !reducedMotion && liveDataEnabled && "animate-spin")}
        />
        <span>Stopping…</span>
      </div>
    );
  }
  if (stopCompletionNote && !running) {
    return (
      <div
        role="status"
        data-testid="session-stop-completion-row"
        data-status="completed"
        className={cn(ROW_CLASS, "text-faint")}
      >
        <span>
          Completed
          <SessionStatusSeparator />
          the stop arrived after the turn finished
        </span>
      </div>
    );
  }
  return (
    <SessionStatusLine
      status={status}
      liveDataEnabled={liveDataEnabled}
      pausedAtMs={pausedAtMs}
      reducedMotion={reducedMotion}
      activityDetail={
        session.pending_interactions.length === 0 &&
        (session.supervision?.work_signals.filter(signal => signal.kind === "tool_running")
          .length ?? 0) <= 1
          ? session.activity?.current_tool
          : undefined
      }
    />
  );
}
