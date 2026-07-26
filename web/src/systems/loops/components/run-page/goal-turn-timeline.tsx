import type { ComponentProps } from "react";
import { CheckCircle2, CircleAlert, Clock3, FileWarning, ShieldAlert } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  cn,
  Eyebrow,
  MonoId,
  Pill,
  PillDot,
  Time,
  Timeline,
  TimelineEvent,
  type PillTone,
} from "@agh/ui";

import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";

export interface GoalTurnTimelineProps extends Omit<ComponentProps<"section">, "children"> {
  turns: readonly GoalTurnTimelineItem[];
  live?: boolean;
}

const RESULT_TONE: Record<string, PillTone> = {
  completed: "success",
  "invalid-result": "warning",
  failed: "danger",
  ambiguous: "warning",
};

function sentenceCase(value: string): string {
  const normalized = value.replaceAll("_", " ").replaceAll("-", " ");
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
}

function resultTone(turn: GoalTurnTimelineItem): PillTone {
  return turn.resultStatus === null ? "accent" : (RESULT_TONE[turn.resultStatus] ?? "neutral");
}

function ResultIcon({ turn }: { turn: GoalTurnTimelineItem }) {
  if (turn.resultStatus === null) return <Clock3 className="size-3" aria-hidden="true" />;
  if (turn.resultStatus === "completed") {
    return <CheckCircle2 className="size-3" aria-hidden="true" />;
  }
  if (turn.resultStatus === "failed") {
    return <CircleAlert className="size-3" aria-hidden="true" />;
  }
  if (turn.resultStatus === "ambiguous") {
    return <ShieldAlert className="size-3" aria-hidden="true" />;
  }
  return <FileWarning className="size-3" aria-hidden="true" />;
}

function TurnFact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <dt>
        <Eyebrow className="text-faint">{label}</Eyebrow>
      </dt>
      <dd className="mt-1 min-w-0 text-form-label text-muted">{children}</dd>
    </div>
  );
}

function GoalTurnRow({ turn }: { turn: GoalTurnTimelineItem }) {
  const tone = resultTone(turn);
  const resultLabel = turn.resultStatus ? sentenceCase(turn.resultStatus) : "In progress";
  const verdictLabel = turn.verdictOutcome ? sentenceCase(turn.verdictOutcome) : "Not evaluated";
  const stopReasonLabel = turn.stopReason ? sentenceCase(turn.stopReason) : "Not reported";

  return (
    <TimelineEvent
      tone={tone}
      title={`Turn ${turn.turn}`}
      time={<Time iso={turn.endedAt ?? turn.startedAt} />}
      meta={
        <>
          <Pill tone={tone} size="xs">
            <ResultIcon turn={turn} />
            {resultLabel}
          </Pill>
          <span className="font-mono text-mono-id text-faint">seq {turn.seq}</span>
          <span className="font-mono text-mono-id text-faint">
            generation {turn.generation} · item {turn.itemIndex}
          </span>
        </>
      }
      className="pb-1"
      data-turn-seq={turn.seq}
      data-result-status={turn.resultStatus ?? "open"}
    >
      <dl className="mt-1.5 grid gap-3 border-t border-line-soft pt-2.5 sm:grid-cols-3">
        <TurnFact label="Stop reason">
          <span className={turn.stopReason ? "text-fg" : "text-faint"}>{stopReasonLabel}</span>
        </TurnFact>
        <TurnFact label="Verdict">
          <span className={turn.verdictOutcome ? "text-fg" : "text-faint"}>{verdictLabel}</span>
        </TurnFact>
        <TurnFact label="Session">
          <Link
            className="inline-flex min-h-6 min-w-0 max-w-full items-center rounded-xs hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring"
            data-testid={`goal-turn-session-link-${turn.seq}`}
            params={{ id: turn.sessionId }}
            to="/session/$id"
          >
            <MonoId value={turn.sessionId} className="max-w-full text-fg" />
          </Link>
        </TurnFact>
      </dl>

      {turn.blockingIssues.length > 0 ? (
        <div className="mt-3 rounded-sm bg-warning-tint px-2.5 py-2.5">
          <div className="flex items-center gap-2 text-warning">
            <CircleAlert className="size-3.5" aria-hidden="true" />
            <Eyebrow className="text-warning">Blocking issues</Eyebrow>
          </div>
          <ul className="mt-1.5 divide-y divide-line-soft" aria-label="Blocking issues">
            {turn.blockingIssues.map(issue => (
              <li
                key={issue.id}
                className="flex min-w-0 gap-2 py-1.5 text-form-label first:pt-0 last:pb-0"
              >
                <MonoId value={issue.id} className="shrink-0 text-warning" />
                <span className="min-w-0 text-pretty text-muted">{issue.note}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      <div className="mt-2.5 flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 border-t border-line-soft pt-2 text-form-hint text-subtle">
        {turn.evidenceRef ? (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            Evidence
            <MonoId value={turn.evidenceRef} className="max-w-table-cell-sm text-info" />
          </span>
        ) : (
          <span className="text-faint">No evidence reference</span>
        )}
        {turn.reasonCode ? (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            Cause
            <MonoId value={turn.reasonCode} className="max-w-table-cell-sm text-warning" />
          </span>
        ) : null}
        <span className="ml-auto font-mono text-mono-id tabular-nums text-faint">
          {turn.tokensUsed === null
            ? "tokens not reported"
            : `${turn.tokensUsed.toLocaleString()} tokens`}
        </span>
      </div>
    </TimelineEvent>
  );
}

/** Run-wide Goal turn audit, preserving sequence order and every nullable outcome. */
export function GoalTurnTimeline({
  turns,
  live = false,
  className,
  ...props
}: GoalTurnTimelineProps) {
  const orderedTurns = [...turns].sort((left, right) => left.seq - right.seq);
  return (
    <section
      aria-label="Goal turn timeline"
      data-live={live ? "true" : "false"}
      className={cn("min-w-0 border-t border-line-soft pt-3", className)}
      {...props}
    >
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <Eyebrow className="text-faint">Goal turns</Eyebrow>
        <Pill tone={live ? "accent" : "neutral"} size="xs" pulse={live}>
          <PillDot />
          {live ? "live" : "audit"}
        </Pill>
        <span className="font-mono text-mono-id tabular-nums text-subtle">
          {orderedTurns.length} {orderedTurns.length === 1 ? "turn" : "turns"}
        </span>
      </div>
      {orderedTurns.length > 0 ? (
        <Timeline ariaLabel="Goal turns in run sequence order">
          {orderedTurns.map(turn => (
            <GoalTurnRow key={turn.key} turn={turn} />
          ))}
        </Timeline>
      ) : (
        <p className="rounded-sm bg-neutral-tint px-3 py-2.5 text-form-label text-muted">
          No Goal turns yet. The first turn appears after a work prompt is claimed.
        </p>
      )}
    </section>
  );
}
