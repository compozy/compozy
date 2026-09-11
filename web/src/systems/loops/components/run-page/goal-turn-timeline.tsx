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
} from "@compozy/ui";

import type { GoalTurn } from "../../types";

export interface GoalTurnTimelineProps extends Omit<ComponentProps<"section">, "children"> {
  turns: readonly GoalTurn[];
  live?: boolean;
}

const RESULT_TONE: Record<string, PillTone> = {
  completed: "success",
  "invalid-result": "warning",
  failed: "danger",
  ambiguous: "warning",
};

/** Formats machine result tokens as readable labels without changing their meaning. */
function sentenceCase(value: string): string {
  const normalized = value.replaceAll("_", " ").replaceAll("-", " ");
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
}

/** Maps reported turn outcomes to semantic tones, retaining neutral styling for unknown outcomes. */
function resultTone(turn: GoalTurn): PillTone {
  return turn.result_status === null ? "accent" : (RESULT_TONE[turn.result_status] ?? "neutral");
}

/** Distinguishes pending, completed, failed, and uncertain turn outcomes visually. */
function ResultIcon({ turn }: { turn: GoalTurn }) {
  if (turn.result_status === null) return <Clock3 className="size-3" aria-hidden="true" />;
  if (turn.result_status === "completed") {
    return <CheckCircle2 className="size-3" aria-hidden="true" />;
  }
  if (turn.result_status === "failed") {
    return <CircleAlert className="size-3" aria-hidden="true" />;
  }
  if (turn.result_status === "ambiguous") {
    return <ShieldAlert className="size-3" aria-hidden="true" />;
  }
  return <FileWarning className="size-3" aria-hidden="true" />;
}

/** Pairs a structural label with its operational value in the turn details. */
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

/** The stream keeps its canonical name in code; the label reads plainly. */
const STREAM_LABEL: Record<"stdout" | "stderr", string> = {
  stdout: "Output",
  stderr: "Errors",
};

/** Renders available judge output streams with bounded scrolling and preserved whitespace. */
function CriterionOutput({ label, value }: { label: "stdout" | "stderr"; value?: string }) {
  if (!value) return null;
  return (
    <div className="mt-2 min-w-0">
      <Eyebrow className={label === "stderr" ? "text-danger" : "text-faint"}>
        {STREAM_LABEL[label]}
      </Eyebrow>
      <pre
        className={cn(
          "mt-1 max-h-40 overflow-auto rounded-xs px-2 py-1.5 font-mono text-mono-id whitespace-pre-wrap break-words",
          label === "stderr" ? "bg-danger-tint text-danger" : "bg-neutral-tint text-muted"
        )}
      >
        {value}
      </pre>
    </div>
  );
}

/** Shows judge criteria, their output, and warnings without hiding failed evaluations. */
function GoalTurnDiagnostics({ turn }: { turn: GoalTurn }) {
  if (turn.criteria.length === 0 && turn.warnings.length === 0) return null;
  return (
    <div className="mt-3 border-t border-line-soft pt-2.5" data-testid="goal-turn-diagnostics">
      <Eyebrow className="text-faint">Judge diagnostics</Eyebrow>
      {turn.criteria.length > 0 ? (
        <ul className="mt-1.5 divide-y divide-line-soft" aria-label="Judge criteria">
          {turn.criteria.map(criterion => (
            <li key={criterion.id} className="min-w-0 py-2 first:pt-0 last:pb-0">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <MonoId value={criterion.id} className="max-w-table-cell-sm text-fg" />
                <Pill tone="neutral" size="xs">
                  {criterion.type}
                </Pill>
                <Pill tone={criterion.passed ? "success" : "warning"} size="xs">
                  {sentenceCase(criterion.outcome)}
                </Pill>
                {criterion.exit_code !== undefined ? (
                  <span className="font-mono text-mono-id text-subtle">
                    exit {criterion.exit_code === null ? "not reported" : criterion.exit_code}
                  </span>
                ) : null}
              </div>
              <CriterionOutput label="stdout" value={criterion.stdout} />
              <CriterionOutput label="stderr" value={criterion.stderr} />
              {criterion.blocking_issues?.length ? (
                <ul className="mt-2 space-y-1" aria-label={`${criterion.id} blocking issues`}>
                  {criterion.blocking_issues.map(issue => (
                    <li key={issue.id} className="flex min-w-0 gap-2 text-form-label text-muted">
                      <MonoId value={issue.id} className="shrink-0 text-warning" />
                      <span className="min-w-0 text-pretty">{issue.note}</span>
                    </li>
                  ))}
                </ul>
              ) : null}
              {criterion.warnings?.length ? (
                <ul className="mt-2 space-y-1" aria-label={`${criterion.id} warnings`}>
                  {criterion.warnings.map(warning => (
                    <li key={`${warning.code}:${warning.message}`} className="text-form-label">
                      <MonoId value={warning.code} className="mr-2 text-warning" />
                      <span className="text-muted">{warning.message}</span>
                    </li>
                  ))}
                </ul>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {turn.warnings.length > 0 ? (
        <ul className="mt-2 space-y-1 border-t border-line-soft pt-2" aria-label="Judge warnings">
          {turn.warnings.map(warning => (
            <li key={`${warning.code}:${warning.message}`} className="text-form-label">
              <MonoId value={warning.code} className="mr-2 text-warning" />
              <span className="text-muted">{warning.message}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** Presents one historical Goal turn with its outcome, blockers, evidence, and usage. */
function GoalTurnRow({ turn }: { turn: GoalTurn }) {
  const tone = resultTone(turn);
  const resultLabel = turn.result_status ? sentenceCase(turn.result_status) : "In progress";
  const verdictLabel = turn.verdict_outcome ? sentenceCase(turn.verdict_outcome) : "Not evaluated";
  const stopReasonLabel = turn.stop_reason ? sentenceCase(turn.stop_reason) : "Not reported";

  return (
    <TimelineEvent
      tone={tone}
      title={`Turn ${turn.turn}`}
      time={<Time iso={turn.ended_at ?? turn.started_at} />}
      meta={
        <>
          <Pill tone={tone} size="xs">
            <ResultIcon turn={turn} />
            {resultLabel}
          </Pill>
          <span className="font-mono text-mono-id text-faint">seq {turn.seq}</span>
          <span className="font-mono text-mono-id text-faint">
            generation {turn.generation} · item {turn.item_index}
          </span>
        </>
      }
      className="pb-1"
      data-turn-seq={turn.seq}
      data-result-status={turn.result_status ?? "open"}
    >
      <dl className="mt-1.5 grid gap-3 border-t border-line-soft pt-2.5 sm:grid-cols-3">
        <TurnFact label="Stop reason">
          <span className={turn.stop_reason ? "text-fg" : "text-faint"}>{stopReasonLabel}</span>
        </TurnFact>
        <TurnFact label="Verdict">
          <span className={turn.verdict_outcome ? "text-fg" : "text-faint"}>{verdictLabel}</span>
        </TurnFact>
        <TurnFact label="Session">
          <Link
            className="inline-flex min-h-6 min-w-0 max-w-full items-center rounded-xs hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring"
            data-testid={`goal-turn-session-link-${turn.seq}`}
            params={{ id: turn.session_id }}
            to="/session/$id"
          >
            <MonoId value={turn.session_id} className="max-w-full text-fg" />
          </Link>
        </TurnFact>
      </dl>

      {turn.blocking_issues.length > 0 ? (
        <div className="mt-3 rounded-sm bg-warning-tint px-2.5 py-2.5">
          <div className="flex items-center gap-2 text-warning">
            <CircleAlert className="size-3.5" aria-hidden="true" />
            <Eyebrow className="text-warning">Blocking issues</Eyebrow>
          </div>
          <ul className="mt-1.5 divide-y divide-line-soft" aria-label="Blocking issues">
            {turn.blocking_issues.map(issue => (
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

      <GoalTurnDiagnostics turn={turn} />

      <div className="mt-2.5 flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 border-t border-line-soft pt-2 text-form-hint text-subtle">
        {turn.evidence_ref ? (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            <Eyebrow>Evidence</Eyebrow>
            <MonoId value={turn.evidence_ref} className="max-w-table-cell-sm text-info" />
          </span>
        ) : (
          <span className="text-faint">No evidence reference</span>
        )}
        {turn.reason_code ? (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            <Eyebrow>Cause</Eyebrow>
            <MonoId value={turn.reason_code} className="max-w-table-cell-sm text-warning" />
          </span>
        ) : null}
        <span className="ml-auto font-mono text-mono-id tabular-nums text-faint">
          {turn.tokens_used === null
            ? "tokens not reported"
            : `${turn.tokens_used.toLocaleString()} tokens`}
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
          {turns.length} {turns.length === 1 ? "turn" : "turns"}
        </span>
      </div>
      {turns.length > 0 ? (
        <Timeline ariaLabel="Goal turns in run sequence order">
          {turns.map(turn => (
            <GoalTurnRow key={turn.seq} turn={turn} />
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
