import { Check, CircleSlash, RotateCcw } from "lucide-react";
import { useState } from "react";

import { Button, Pill, StatusCard, type StatusCardTone } from "@compozy/ui";

import type { LoopCoordinatorFailure } from "../../lib/loop-events";
import { loopRunBestLabel } from "../../lib/loop-generation-presentation";
import { formatClockDuration, terminalRunElapsedSeconds } from "../../lib/loop-run-usage";
import type { LoopRunRecord } from "../../types";
import { LoopSection } from "../loop-section";
import { LoopRunQuietNote } from "./loop-run-quiet-note";

interface LoopRunOutcomeCardProps {
  run: LoopRunRecord;
  /** Run-level failure from the terminal status frame; null when none streamed. */
  failure: LoopCoordinatorFailure | null;
  /** The terminal transition's `from` status, read from the retained frame. */
  fromStatus?: string;
  /** The terminal transition's `at`; drives the true frozen duration + Time-limit check. */
  terminalAt?: string;
  /**
   * The terminal transition's `cause` (`operator_cancel` / `operator_kill` / …).
   * It is the only thing that distinguishes a cooperative cancel from a kill —
   * both statuses read `canceled`.
   */
  cause?: string;
  /** `contract.no_progress.window` from the pinned definition (stalled card). */
  noProgressWindow?: number;
  /** Blocking-issue ids from the last check (stalled card evidence). */
  repeatedIssueIds: string[];
  onStartNewRun: () => void;
  isStartPending?: boolean;
}

interface OutcomeView {
  sectionLabel: string | null;
  tone: StatusCardTone;
  cardClass: string;
  titleClass: string;
  title: string;
  body?: string;
  recovery?: string;
  showStartNewRun: boolean;
}

/** Names only a cap the run actually crossed; exhausted also has non-budget causes. */
function exhaustedLimit(run: LoopRunRecord, elapsedSeconds: number): string | null {
  if (run.iteration_cap > 0 && run.generation >= run.iteration_cap) return "Round cap";
  if (run.budget_tokens > 0 && run.tokens_used >= run.budget_tokens) return "Token budget";
  if (run.budget_wall_sec > 0 && elapsedSeconds >= run.budget_wall_sec) return "Time limit";
  return null;
}

function outcomeView(
  props: LoopRunOutcomeCardProps,
  elapsedSeconds: number,
  durationLabel: string
): OutcomeView | null {
  const { run, failure, noProgressWindow } = props;
  switch (run.status) {
    case "failed":
      return {
        sectionLabel: "What went wrong",
        tone: "danger",
        cardClass: "border border-danger/25 bg-danger-tint",
        titleClass: "text-danger",
        title: `Failed after ${durationLabel}`,
        body: failure?.cause,
        recovery: failure?.recovery,
        showStartNewRun: true,
      };
    case "blocked":
      return {
        sectionLabel: "Why it stopped",
        tone: "warning",
        cardClass: "border border-warning/25 bg-warning-tint",
        titleClass: "text-warning",
        title: "Blocked on something outside the run",
        body:
          failure?.cause ?? "The run stopped on an external dependency it cannot resolve itself.",
        recovery: failure?.recovery,
        showStartNewRun: false,
      };
    case "exhausted": {
      const limit = exhaustedLimit(run, elapsedSeconds);
      return {
        sectionLabel: "Why it stopped",
        tone: "warning",
        cardClass: "border border-warning/25 bg-warning-tint",
        titleClass: "text-warning",
        title: failure?.cause
          ? `${failure.cause} after ${durationLabel}`
          : limit
            ? `${limit} reached after ${durationLabel}`
            : `Run exhausted after ${durationLabel}`,
        body:
          failure?.cause ??
          (limit
            ? "This loop's limit policy stops the run as exhausted instead of asking. Finished work is kept."
            : "This run stopped as exhausted. Finished work is kept."),
        recovery: failure?.recovery,
        showStartNewRun: true,
      };
    }
    // Cancel and kill both land on `canceled`; only the terminal frame's cause
    // distinguishes a cooperative wind-down from an immediate stop. The card is
    // deliberately calm: a deliberate end is not a failure.
    case "canceled": {
      const killed = props.cause === "operator_kill";
      return {
        sectionLabel: "Why it stopped",
        tone: "neutral",
        cardClass: "border border-line bg-canvas-tint",
        titleClass: "text-fg-strong",
        title: killed ? "Killed by you" : "Canceled by you",
        body: killed
          ? "Stopped immediately: in-flight work was interrupted mid-step and its reactions were skipped. Finished work is still kept."
          : "You asked the run to stop and it wound down cleanly: in-flight lanes drained, their cleanup reactions ran, and finished work is kept.",
        recovery: failure?.recovery,
        showStartNewRun: true,
      };
    }
    case "stalled":
      return {
        sectionLabel: "Why it stopped",
        tone: "neutral",
        cardClass: "border border-line bg-canvas-tint",
        titleClass: "text-fg-strong",
        title: "Stalled — no progress",
        body: noProgressWindow
          ? `No new progress across ${noProgressWindow} rounds — the same open points kept coming back.`
          : "No new progress across recent rounds — the same open points kept coming back.",
        recovery: failure?.recovery,
        showStartNewRun: false,
      };
    default:
      return null;
  }
}

export function LoopRunOutcomeCard(props: LoopRunOutcomeCardProps) {
  const [renderedAt] = useState(Date.now);
  const {
    run,
    failure,
    fromStatus,
    terminalAt,
    cause,
    repeatedIssueIds,
    onStartNewRun,
    isStartPending,
  } = props;
  if (run.status === "no-op") {
    return (
      <LoopRunQuietNote data-testid="loop-run-noop-note" icon={Check}>
        Ran, nothing to do — the run finished without changes.
      </LoopRunQuietNote>
    );
  }
  const elapsedSeconds = terminalRunElapsedSeconds(run, terminalAt, renderedAt);
  const durationLabel = formatClockDuration(elapsedSeconds);
  const view = outcomeView(props, elapsedSeconds, durationLabel);
  if (!view) return null;
  const best = loopRunBestLabel(run);
  const showBestPointer = (run.status === "exhausted" || run.status === "stalled") && best;
  // One `cause` segment. The terminal frame's own cause wins when it carries
  // one (`operator_cancel` / `operator_kill` — the only thing separating a
  // cancel from a kill); otherwise the coordinator failure code names it.
  const causeLabel = cause || failure?.code;
  const micro = [
    `status_changed · ${fromStatus ?? "?"} → ${run.status}`,
    causeLabel ? `cause ${causeLabel}` : null,
  ]
    .filter(Boolean)
    .join(" · ");
  const card = (
    <StatusCard className={`gap-0 px-4 py-3.5 ${view.cardClass}`} tone={view.tone}>
      <div
        className={`text-ws-name font-medium ${view.titleClass}`}
        data-testid="loop-run-outcome-title"
      >
        {view.title}
      </div>
      {view.body ? (
        <StatusCard.Body className="mt-1 max-w-[62ch] leading-relaxed">{view.body}</StatusCard.Body>
      ) : null}
      {view.recovery ? (
        <StatusCard.Body className="mt-2 max-w-[62ch] leading-relaxed">
          {view.recovery}
        </StatusCard.Body>
      ) : null}
      {run.status === "stalled" && repeatedIssueIds.length > 0 ? (
        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1" data-testid="loop-run-stalled-issues">
          {repeatedIssueIds.map(id => (
            <span key={id} className="font-mono text-mono-id text-subtle">
              {id}
            </span>
          ))}
        </div>
      ) : null}
      {showBestPointer ? (
        <div className="mt-3" data-testid="loop-run-best-pointer">
          <Pill.Link href={`#loop-generation-${run.best_generation}`} size="sm" tone="success">
            Best result · {best}
          </Pill.Link>
        </div>
      ) : null}
      {view.showStartNewRun ? (
        <StatusCard.Action className="mt-3.5">
          <Button
            data-testid="loop-run-start-new"
            disabled={isStartPending}
            onClick={onStartNewRun}
            size="sm"
            type="button"
            variant="primary"
          >
            <RotateCcw aria-hidden="true" className="size-3.5" />
            Start a new run
          </Button>
        </StatusCard.Action>
      ) : null}
      <div className="mt-3 font-mono text-pill-group-badge text-faint">{micro}</div>
    </StatusCard>
  );
  if (!view.sectionLabel) return card;
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-outcome"
      icon={<CircleSlash aria-hidden="true" />}
      title={view.sectionLabel}
    >
      {card}
    </LoopSection>
  );
}
