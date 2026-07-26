import { Check, RotateCcw } from "lucide-react";
import { useState } from "react";

import { Button, StatusCard, type StatusCardTone } from "@agh/ui";

import type { LoopCoordinatorFailure } from "../../lib/loop-events";
import { formatClockDuration, terminalRunElapsedSeconds } from "../../lib/loop-run-usage";
import type { LoopRunRecord } from "../../types";
import { LoopRunQuietNote } from "./loop-run-quiet-note";
import { LoopRunSection } from "./loop-run-section";

interface LoopRunOutcomeCardProps {
  run: LoopRunRecord;
  /** Run-level failure from the terminal status frame; null when none streamed. */
  failure: LoopCoordinatorFailure | null;
  /** The terminal transition's `from` status, read from the retained frame. */
  fromStatus?: string;
  /** The terminal transition's `at`; drives the true frozen duration + Time-limit check. */
  terminalAt?: string;
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

/**
 * The terminal outcome card (§7): "What went wrong" for failed runs plus the
 * blocked / exhausted / stalled explainers and the quiet no-op note. `cause` and
 * `recovery` come from the terminal status frame — never invented; "Start a new
 * run" re-posts the same inputs as a fresh run.
 */
export function LoopRunOutcomeCard(props: LoopRunOutcomeCardProps) {
  const [renderedAt] = useState(Date.now);
  const { run, failure, fromStatus, terminalAt, repeatedIssueIds, onStartNewRun, isStartPending } =
    props;
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
  const micro = [
    `status_changed · ${fromStatus ?? "?"} → ${run.status}`,
    failure ? `cause ${failure.code}` : null,
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
    <LoopRunSection label={view.sectionLabel} data-testid="loop-run-outcome">
      {card}
    </LoopRunSection>
  );
}
