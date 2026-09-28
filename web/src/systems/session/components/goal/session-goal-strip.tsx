import { useState } from "react";
import { ChevronDown, FilePenLine, RefreshCw } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, cn, Eyebrow } from "@compozy/ui";

import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";

export interface SessionGoalStripProps {
  snapshot: SessionGoalSnapshot;
  composerAffordance?: GoalComposerAffordance;
  onPrefillComposer?: (text: string) => void;
}

type GoalStripState = "active" | "paused" | "blocked" | "done" | "moved";

// Dot tones per the strip contract: accent pulse active · warning
// blocked/paused · success done · faint moved. Every non-canonical status
// (usage-limited, budget-limited, unknown) reads as blocked.
function goalStripState(snapshot: SessionGoalSnapshot): GoalStripState {
  if (snapshot.bound_session_id !== snapshot.origin_session_id) return "moved";
  switch (snapshot.status) {
    case "active":
      return "active";
    case "paused":
      return "paused";
    case "complete":
      return "done";
    default:
      return "blocked";
  }
}

const STATE_DOT: Record<GoalStripState, string> = {
  active: "bg-accent session-state-pulse",
  paused: "bg-warning",
  blocked: "bg-warning",
  done: "bg-success",
  moved: "bg-faint",
};

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

/**
 * The goal as one quiet line above the transcript (`.goalstrip`): state dot +
 * GOAL kicker + objective + a plain "Step a of b" fact + chevron, expanding to
 * key/value rows (context usage lives in the composer ring and the body; the
 * node id and cause ride on data attributes). No pills, no meters, no control bar — lifecycle actions
 * live on the window head; the body's Actions row only stages `/goal` commands
 * into the composer, never sends.
 */
export function SessionGoalStrip({
  snapshot,
  composerAffordance,
  onPrefillComposer,
}: SessionGoalStripProps) {
  const [open, setOpen] = useState(false);
  const state = goalStripState(snapshot);
  const moved = state === "moved";
  const facts = `Step ${snapshot.turns_used} of ${snapshot.turn_limit}`;
  const showActions = composerAffordance !== undefined && onPrefillComposer !== undefined;

  return (
    <section
      aria-label="Goal status"
      data-testid="session-goal-strip"
      data-state={state}
      data-goal-status={snapshot.status}
      data-run-status={snapshot.run_status}
      data-live={snapshot.live ? "true" : "false"}
      data-goal-node={snapshot.node_id}
      data-goal-cause={snapshot.cause || undefined}
      className="min-w-0 border-b border-line py-1.5"
    >
      <button
        type="button"
        aria-expanded={open}
        data-testid="goal-strip-line"
        onClick={() => setOpen(value => !value)}
        className={cn(
          "flex min-h-6 w-full min-w-0 items-center gap-2 rounded-md px-1 text-left",
          "transition-colors duration-base ease-out hover:bg-hover",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <span
          aria-hidden="true"
          data-testid="goal-strip-dot"
          className={cn("size-1.5 shrink-0 rounded-full", STATE_DOT[state])}
        />
        <Eyebrow className="shrink-0 text-subtle">Goal</Eyebrow>
        <span className="min-w-0 max-w-sm flex-1 truncate text-small-body text-fg">
          {snapshot.objective}
        </span>
        <span
          aria-atomic="true"
          aria-live="polite"
          data-testid="goal-strip-facts"
          className="shrink-0 text-transcript-meta text-subtle tabular-nums"
        >
          {moved ? `Moved · ${facts}` : facts}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            open ? "rotate-180" : null
          )}
          strokeWidth={1.75}
        />
      </button>
      {open ? (
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
          {snapshot.last_verdict ? (
            <StripRow label="Last verdict">
              {sentenceCase(snapshot.last_verdict.outcome)} ·{" "}
              {snapshot.last_verdict.blocking_issues.length} blocking{" "}
              {snapshot.last_verdict.blocking_issues.length === 1 ? "issue" : "issues"}
              {snapshot.last_verdict.evidence_ref ? (
                <>
                  {" "}
                  <span className="font-mono text-badge text-subtle">
                    {snapshot.last_verdict.evidence_ref}
                  </span>
                </>
              ) : null}
            </StripRow>
          ) : null}
          {moved ? (
            <StripRow label="Session">
              Goal work moved to a new active session.{" "}
              <Link
                to="/session/$id"
                params={{ id: snapshot.bound_session_id }}
                data-testid="goal-strip-active-session"
                className="text-info transition-colors hover:underline hover:underline-offset-2"
              >
                Active session
              </Link>
            </StripRow>
          ) : null}
          {showActions ? (
            <StripRow label="Actions">
              <Button
                type="button"
                variant="ghost"
                size="xs"
                data-testid="goal-strip-prefill"
                onClick={() => onPrefillComposer(prefillCommand(composerAffordance))}
                className="-ml-2 text-muted"
              >
                {composerAffordance.kind === "replace" ? (
                  <RefreshCw aria-hidden="true" className="size-3 text-subtle" />
                ) : (
                  <FilePenLine aria-hidden="true" className="size-3 text-subtle" />
                )}
                {composerAffordance.kind === "replace" ? "Draft replacement" : "Draft goal command"}
              </Button>
            </StripRow>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
