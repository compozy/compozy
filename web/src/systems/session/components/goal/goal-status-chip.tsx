import type { ComponentType } from "react";
import {
  Activity,
  ArrowUpRight,
  CheckCircle2,
  CircleAlert,
  Gauge,
  Link2,
  Pause,
  Target,
} from "lucide-react";
import { Link } from "@tanstack/react-router";

import { buttonVariants, cn, Eyebrow, MonoId, Pill, PillDot, type PillTone } from "@agh/ui";

import { GoalControls } from "./goal-status-controls";
import { GoalContextMeter, GoalTurnMeter } from "./goal-status-meters";
import type { GoalStatusChipProps, SessionGoalSnapshot } from "./goal-status-types";

export type {
  GoalComposerAffordance,
  GoalStatusChipProps,
  SessionGoalSnapshot,
} from "./goal-status-types";

type GoalStatus = "active" | "paused" | "blocked" | "usage-limited" | "budget-limited" | "complete";

interface GoalStatusPresentation {
  icon: ComponentType<{ className?: string }>;
  label: string;
  tone: PillTone;
}

const GOAL_STATUS: Record<GoalStatus, GoalStatusPresentation> = {
  active: { icon: Activity, label: "active", tone: "success" },
  paused: { icon: Pause, label: "paused", tone: "warning" },
  blocked: { icon: CircleAlert, label: "blocked", tone: "danger" },
  "usage-limited": { icon: Gauge, label: "usage-limited", tone: "warning" },
  "budget-limited": { icon: Gauge, label: "budget-limited", tone: "warning" },
  complete: { icon: CheckCircle2, label: "complete", tone: "success" },
};

const GOAL_STATUSES = new Set<GoalStatus>(Object.keys(GOAL_STATUS) as GoalStatus[]);

function goalStatusPresentation(status: string): GoalStatusPresentation {
  if (GOAL_STATUSES.has(status as GoalStatus)) return GOAL_STATUS[status as GoalStatus];
  return { icon: CircleAlert, label: status, tone: "neutral" };
}

function sentenceCase(value: string): string {
  const normalized = value.replaceAll("_", " ").replaceAll("-", " ");
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
}

function isRedundantRunStatus(statusLabel: string, runStatus: string): boolean {
  const normalizedStatus = statusLabel.trim().toLowerCase().replaceAll(" ", "-");
  const normalizedRun = runStatus.trim().toLowerCase().replaceAll(" ", "-");
  if (normalizedStatus === normalizedRun) return true;
  return normalizedStatus === "active" && (normalizedRun === "running" || normalizedRun === "live");
}

function GoalLinks({ snapshot }: { snapshot: SessionGoalSnapshot }) {
  const moved = snapshot.bound_session_id !== snapshot.origin_session_id;
  return (
    <div className="flex flex-wrap items-center gap-1">
      <Link
        to="/loop-runs/$runId"
        params={{ runId: snapshot.run_id }}
        hash={`node-${snapshot.node_id}`}
        className={buttonVariants({ variant: "ghost", size: "xs" })}
      >
        Open run
        <ArrowUpRight data-icon="inline-end" aria-hidden="true" />
      </Link>
      {moved ? (
        <Link
          to="/session/$id"
          params={{ id: snapshot.bound_session_id }}
          className={buttonVariants({ variant: "ghost", size: "xs" })}
        >
          Active session
          <ArrowUpRight data-icon="inline-end" aria-hidden="true" />
        </Link>
      ) : null}
    </div>
  );
}

/**
 * Canonical session-header pointer to the newest session-origin Goal snapshot.
 * It stays present for terminal snapshots and renders only controls backed by callbacks.
 */
export function GoalStatusChip({
  snapshot,
  composerAffordance,
  pendingAction,
  onPause,
  onResume,
  onApprove,
  onClear,
  onPrefillComposer,
  className,
  ...props
}: GoalStatusChipProps) {
  const status = goalStatusPresentation(snapshot.status);
  const StatusIcon = status.icon;
  const moved = snapshot.bound_session_id !== snapshot.origin_session_id;

  return (
    <section
      aria-label="Goal status"
      aria-live="polite"
      data-goal-status={snapshot.status}
      data-run-status={snapshot.run_status}
      data-live={snapshot.live ? "true" : "false"}
      className={cn(
        "overflow-hidden rounded-md border border-line bg-canvas-soft text-fg",
        className
      )}
      {...props}
    >
      <div className="flex min-w-0 flex-col gap-3 px-3 py-3 sm:px-4">
        <div className="flex min-w-0 flex-wrap items-start gap-3">
          <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-icon-well bg-accent-tint text-accent">
            <Target className="size-4" aria-hidden="true" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <Eyebrow className="text-muted">Goal</Eyebrow>
              <Pill
                tone={status.tone}
                size="sm"
                pulse={snapshot.live && snapshot.status === "active"}
              >
                <StatusIcon className="size-3" aria-hidden="true" />
                {status.label}
              </Pill>
              {!isRedundantRunStatus(status.label, snapshot.run_status) ? (
                <Pill tone="neutral" size="xs">
                  <PillDot tone={snapshot.live ? "success" : "neutral"} />
                  {sentenceCase(snapshot.run_status)}
                </Pill>
              ) : null}
            </div>
            <p className="mt-1.5 max-w-[75ch] text-small-body font-medium text-pretty text-fg-strong">
              {snapshot.objective}
            </p>
            {snapshot.contract_summary ? (
              <p className="mt-1 max-w-[75ch] text-form-hint text-pretty text-muted">
                {snapshot.contract_summary}
              </p>
            ) : null}
          </div>
          <GoalLinks snapshot={snapshot} />
        </div>

        {moved ? (
          <div className="flex flex-wrap items-center gap-2 rounded-sm bg-info-tint px-2.5 py-2 text-form-hint text-info">
            <Link2 className="size-3.5 shrink-0" aria-hidden="true" />
            <span>Goal work moved to a new active session.</span>
            <MonoId value={snapshot.bound_session_id} className="text-info" />
          </div>
        ) : null}
      </div>

      <div className="grid border-y border-line-soft sm:grid-cols-3 sm:divide-x sm:divide-line-soft">
        <div className="px-3 py-3 sm:px-4">
          <GoalTurnMeter snapshot={snapshot} />
        </div>
        <div className="border-t border-line-soft px-3 py-3 sm:border-t-0 sm:px-4">
          <GoalContextMeter context={snapshot.context} />
        </div>
        <div className="flex min-w-0 flex-col gap-2 border-t border-line-soft px-3 py-3 sm:border-t-0 sm:px-4">
          <div className="flex items-center justify-between gap-3">
            <Eyebrow className="text-muted">Run</Eyebrow>
            <Pill tone={snapshot.live ? "success" : "neutral"} size="xs">
              <PillDot />
              {snapshot.live ? "live" : "settled"}
            </Pill>
          </div>
          <div className="flex min-w-0 items-center gap-2 text-form-hint text-muted">
            <span>Node</span>
            <MonoId value={snapshot.node_id} className="min-w-0 text-fg" />
          </div>
          {snapshot.cause ? (
            <div className="flex min-w-0 items-center gap-2 text-form-hint text-muted">
              <span>Cause</span>
              <MonoId value={snapshot.cause} className="min-w-0 text-warning" />
            </div>
          ) : (
            <p className="text-form-hint text-muted">No transition cause reported.</p>
          )}
        </div>
      </div>

      {snapshot.last_verdict ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2.5 text-form-hint text-muted sm:px-4">
          <Eyebrow className="text-muted">Last verdict</Eyebrow>
          <span className="font-medium text-fg">{sentenceCase(snapshot.last_verdict.outcome)}</span>
          <span>
            {snapshot.last_verdict.blocking_issues.length} blocking{" "}
            {snapshot.last_verdict.blocking_issues.length === 1 ? "issue" : "issues"}
          </span>
          {snapshot.last_verdict.evidence_ref ? (
            <MonoId value={snapshot.last_verdict.evidence_ref} className="text-info" />
          ) : null}
        </div>
      ) : null}

      <GoalControls
        snapshot={snapshot}
        composerAffordance={composerAffordance}
        pendingAction={pendingAction}
        onPause={onPause}
        onResume={onResume}
        onApprove={onApprove}
        onClear={onClear}
        onPrefillComposer={onPrefillComposer}
      />
    </section>
  );
}
