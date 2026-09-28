import { FilePenLine, RefreshCw } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button } from "@compozy/ui";

import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";

function clampRatio(value: number): number {
  return Math.max(0, Math.min(1, value));
}

function formatPercent(value: number): string {
  return `${Math.round(clampRatio(value) * 100)}%`;
}

function sentenceCase(value: string): string {
  const normalized = value.replaceAll("_", " ").replaceAll("-", " ");
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
}

function prefillCommand(affordance: GoalComposerAffordance): string {
  if (affordance.kind === "replace") {
    return `/goal replace ${affordance.expectedRunId} ${affordance.objective}`;
  }
  return `/goal ${affordance.expandedObjective}`;
}

/** Displays one expanded Goal field, allowing long values to wrap within the strip. */
function StripRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2.5 text-transcript-body leading-normal">
      <span className="w-20 shrink-0 pt-px text-transcript-caption text-faint">{label}</span>
      <span className="min-w-0 whitespace-pre-wrap text-muted [overflow-wrap:anywhere]">
        {children}
      </span>
    </div>
  );
}

function ContextRow({ context }: { context: SessionGoalSnapshot["context"] }) {
  if (context.state === "pending") {
    return <StripRow label="Context">Waiting for a newer usage report.</StripRow>;
  }
  if (context.state === "unknown") {
    return <StripRow label="Context">This agent has not reported context usage.</StripRow>;
  }
  const threshold =
    context.nudge_ratio === 0 ? "nudge disabled" : `nudge at ${formatPercent(context.nudge_ratio)}`;
  const ratioLabel =
    context.ratio !== null ? `${formatPercent(context.ratio)} used` : "usage known";
  return (
    <StripRow label="Context">
      <span className="font-mono text-badge text-subtle tabular-nums">
        {context.used !== null && context.size !== null
          ? `${context.used.toLocaleString()} / ${context.size.toLocaleString()} tokens · ${threshold}`
          : `${ratioLabel} · ${threshold}`}
      </span>
    </StripRow>
  );
}

function LastVerdictRow({
  verdict,
}: {
  verdict: NonNullable<SessionGoalSnapshot["last_verdict"]>;
}) {
  return (
    <StripRow label="Last verdict">
      {sentenceCase(verdict.outcome)} · {verdict.blocking_issues.length} blocking{" "}
      {verdict.blocking_issues.length === 1 ? "issue" : "issues"}
      {verdict.evidence_ref ? (
        <>
          {" "}
          <span className="font-mono text-badge text-subtle">{verdict.evidence_ref}</span>
        </>
      ) : null}
    </StripRow>
  );
}

function MovedSessionRow({ sessionId }: { sessionId: string }) {
  return (
    <StripRow label="Session">
      Goal work moved to a new active session.{" "}
      <Link
        to="/session/$id"
        params={{ id: sessionId }}
        data-testid="goal-strip-active-session"
        className="text-info transition-colors hover:underline hover:underline-offset-2"
      >
        Active session
      </Link>
    </StripRow>
  );
}

function PrefillActionRow({
  affordance,
  onPrefillComposer,
}: {
  affordance: GoalComposerAffordance;
  onPrefillComposer: (text: string) => void;
}) {
  const replace = affordance.kind === "replace";
  const Icon = replace ? RefreshCw : FilePenLine;
  return (
    <StripRow label="Actions">
      <Button
        type="button"
        variant="ghost"
        size="xs"
        data-testid="goal-strip-prefill"
        onClick={() => onPrefillComposer(prefillCommand(affordance))}
        className="-ml-2 text-muted"
      >
        <Icon aria-hidden="true" className="size-3 text-subtle" />
        {replace ? "Draft replacement" : "Draft goal command"}
      </Button>
    </StripRow>
  );
}

export interface SessionGoalStripBodyProps {
  snapshot: SessionGoalSnapshot;
  moved: boolean;
  composerAffordance?: GoalComposerAffordance;
  onPrefillComposer?: (text: string) => void;
}

/** The expanded strip: the goal's fields as key/value rows plus the staging action. */
export function SessionGoalStripBody({
  snapshot,
  moved,
  composerAffordance,
  onPrefillComposer,
}: SessionGoalStripBodyProps) {
  return (
    <div data-testid="goal-strip-body" className="flex flex-col gap-1.5 px-1 pt-1.5 pb-0.5">
      {/* The line truncates the objective; the body is where it reads in full. */}
      <StripRow label="Objective">{snapshot.objective}</StripRow>
      {snapshot.contract_summary ? (
        <StripRow label="Contract">{snapshot.contract_summary}</StripRow>
      ) : null}
      <StripRow label="Run">
        <Link
          to="/loop-runs/$runId"
          params={{ runId: snapshot.run_id }}
          hash={`node-${snapshot.node_id}`}
          className="text-info transition-colors hover:underline hover:underline-offset-2"
        >
          Open run
        </Link>{" "}
        · {sentenceCase(snapshot.run_status)} · {snapshot.live ? "live" : "settled"}
      </StripRow>
      <ContextRow context={snapshot.context} />
      {snapshot.last_verdict ? <LastVerdictRow verdict={snapshot.last_verdict} /> : null}
      {moved ? <MovedSessionRow sessionId={snapshot.bound_session_id} /> : null}
      {composerAffordance !== undefined && onPrefillComposer !== undefined ? (
        <PrefillActionRow affordance={composerAffordance} onPrefillComposer={onPrefillComposer} />
      ) : null}
    </div>
  );
}
