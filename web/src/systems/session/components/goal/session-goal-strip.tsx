import { useState } from "react";
import { ChevronDown, FilePenLine, RefreshCw } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { cn, Eyebrow } from "@compozy/ui";

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

function contextFact(context: SessionGoalSnapshot["context"]): string {
  if (context.state === "known" && context.ratio !== null) {
    return `ctx ${formatPercent(context.ratio)}`;
  }
  return "ctx —";
}

function prefillCommand(affordance: GoalComposerAffordance): string {
  if (affordance.kind === "replace") {
    return `/goal replace ${affordance.expectedRunId} ${affordance.objective}`;
  }
  return `/goal ${affordance.expandedObjective}`;
}

function StripRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2.5 text-[12px] leading-normal">
      <span className="w-[76px] shrink-0 pt-px text-[11px] text-faint">{label}</span>
      <span className="min-w-0 text-muted">{children}</span>
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
 * GOAL kicker + objective + `turn a/b · ctx n%` mono facts + chevron, expanding
 * to key/value rows. No pills, no meters, no control bar — lifecycle actions
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
  const facts = `turn ${snapshot.turns_used}/${snapshot.turn_limit} · ${contextFact(snapshot.context)}`;
  const showActions = composerAffordance !== undefined && onPrefillComposer !== undefined;

  return (
    <section
      aria-label="Goal status"
      aria-live="polite"
      data-testid="session-goal-strip"
      data-state={state}
      data-goal-status={snapshot.status}
      data-run-status={snapshot.run_status}
      data-live={snapshot.live ? "true" : "false"}
      className="min-w-0 border-b border-line pt-[5px] pb-[7px]"
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
        <span className="min-w-0 flex-1 truncate text-small-body text-fg">
          {snapshot.objective}
        </span>
        <span
          data-testid="goal-strip-facts"
          className="shrink-0 font-mono text-badge text-subtle tabular-nums"
        >
          {moved ? `moved · ${facts}` : facts}
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
        <div data-testid="goal-strip-body" className="flex flex-col gap-[5px] px-1 pt-1.5 pb-0.5">
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
          <StripRow label="Node">
            <span className="font-mono text-badge text-subtle">{snapshot.node_id}</span>
            {snapshot.cause ? (
              <>
                {" "}
                · <span className="font-mono text-badge text-warning">{snapshot.cause}</span>
              </>
            ) : null}
          </StripRow>
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
              <span className="flex items-center gap-1.5">
                <button
                  type="button"
                  data-testid="goal-strip-prefill"
                  onClick={() => onPrefillComposer(prefillCommand(composerAffordance))}
                  className={cn(
                    "inline-flex min-h-[22px] items-center gap-[5px] rounded-xs border border-transparent px-2",
                    "text-[11.5px] text-muted transition-colors duration-base ease-out",
                    "hover:border-line hover:bg-hover hover:text-fg",
                    "focus-visible:shadow-focus-ring focus-visible:outline-none"
                  )}
                >
                  {composerAffordance.kind === "replace" ? (
                    <RefreshCw aria-hidden="true" className="size-[11px] text-subtle" />
                  ) : (
                    <FilePenLine aria-hidden="true" className="size-[11px] text-subtle" />
                  )}
                  {composerAffordance.kind === "replace"
                    ? "Draft replacement"
                    : "Draft goal command"}
                </button>
              </span>
            </StripRow>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
