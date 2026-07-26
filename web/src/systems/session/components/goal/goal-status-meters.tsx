import { cn, Eyebrow, Pill, PillDot, type PillTone } from "@agh/ui";

import type { SessionGoalSnapshot } from "./goal-status-types";

function clampRatio(value: number): number {
  return Math.max(0, Math.min(1, value));
}

function formatPercent(value: number): string {
  return `${Math.round(clampRatio(value) * 100)}%`;
}

export function GoalContextMeter({ context }: { context: SessionGoalSnapshot["context"] }) {
  const ratio = context.state === "known" ? context.ratio : null;
  const warns = ratio !== null && context.nudge_ratio > 0 && ratio >= context.nudge_ratio;
  const tone: PillTone =
    context.state === "pending"
      ? "info"
      : context.state === "unknown"
        ? "neutral"
        : warns
          ? "warning"
          : "success";
  const valueLabel = ratio === null ? context.state : formatPercent(ratio);
  const thresholdLabel =
    context.nudge_ratio === 0 ? "Nudge disabled" : `Nudge at ${formatPercent(context.nudge_ratio)}`;

  return (
    <div className="flex min-w-0 flex-col gap-2" data-context-state={context.state}>
      <div className="flex items-center justify-between gap-3">
        <Eyebrow className="text-muted">Context</Eyebrow>
        <Pill tone={tone} size="xs">
          <PillDot />
          {valueLabel}
        </Pill>
      </div>
      {ratio !== null ? (
        <div
          role="progressbar"
          aria-label={`Context used ${formatPercent(ratio)}`}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(clampRatio(ratio) * 100)}
          className="h-1.5 overflow-hidden rounded-pill bg-bar-fill"
        >
          <div
            aria-hidden="true"
            className={cn("h-full rounded-pill", warns ? "bg-warning" : "bg-success")}
            style={{ width: formatPercent(ratio) }}
          />
        </div>
      ) : (
        <p className="text-form-hint text-muted">
          {context.state === "pending"
            ? "Waiting for a newer usage report."
            : "This agent has not reported context usage."}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-mono-id text-subtle">
        <span>{thresholdLabel}</span>
        {ratio !== null && context.used !== null && context.size !== null ? (
          <span className="tabular-nums">
            {context.used.toLocaleString()} / {context.size.toLocaleString()} tokens
          </span>
        ) : null}
      </div>
    </div>
  );
}

export function GoalTurnMeter({ snapshot }: { snapshot: SessionGoalSnapshot }) {
  const ratio = snapshot.turn_limit > 0 ? snapshot.turns_used / snapshot.turn_limit : 0;
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex items-baseline justify-between gap-3">
        <Eyebrow className="text-muted">Turns</Eyebrow>
        <span className="font-mono text-small-body font-semibold tabular-nums text-fg-strong">
          {snapshot.turns_used}
          <span className="font-normal text-muted"> / {snapshot.turn_limit}</span>
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={`${snapshot.turns_used} of ${snapshot.turn_limit} Goal turns used`}
        aria-valuemin={0}
        aria-valuemax={snapshot.turn_limit}
        aria-valuenow={Math.min(snapshot.turns_used, snapshot.turn_limit)}
        className="h-1.5 overflow-hidden rounded-pill bg-bar-fill"
      >
        <div
          aria-hidden="true"
          className={cn(
            "h-full rounded-pill",
            snapshot.turns_used >= snapshot.turn_limit ? "bg-warning" : "bg-info"
          )}
          style={{ width: formatPercent(ratio) }}
        />
      </div>
      <p className="text-form-hint text-muted">Effective limit for this Goal.</p>
    </div>
  );
}
