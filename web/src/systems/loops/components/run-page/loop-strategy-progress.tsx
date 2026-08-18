import {
  CircleAlert,
  Flag,
  GitFork,
  ListChecks,
  type LucideIcon,
  Percent,
  Zap,
} from "lucide-react";

import { cn, Eyebrow, Pill } from "@compozy/ui";

import { LOOP_ABSENCE_SIGNALS, LOOP_PARTIAL_SIGNAL } from "../../lib/loop-request-vocabulary";
import type { LoopStrategyCounts, LoopStrategyProgressModel } from "../../lib/loop-run-strategy";

const PARTIAL = LOOP_PARTIAL_SIGNAL;
const CANCELED = LOOP_ABSENCE_SIGNALS.canceled_by_strategy;
const NEVER_MATERIALIZED = LOOP_ABSENCE_SIGNALS.never_materialized;

const STRATEGY_GLYPH: Record<string, LucideIcon> = {
  wait_all: ListChecks,
  best_effort: Percent,
  fail_fast: Zap,
  race: Flag,
};

const PLAIN_FIGURE = "text-fg";
const CALM_FIGURE = "text-muted";
const FAILED_FIGURE = "text-danger";

interface StrategyFigure {
  key: string;
  label: string;
  value: number;

  icon?: LucideIcon;
  className: string;
  testId?: string;
}

function strategyFigures(counts: LoopStrategyCounts): StrategyFigure[] {
  const figures: StrategyFigure[] = [
    { key: "settled", label: "Settled", value: counts.settled, className: PLAIN_FIGURE },
    {
      key: "succeeded",
      label: "Succeeded",
      value: counts.succeeded,
      className: PLAIN_FIGURE,
      testId: "loop-strategy-count-succeeded",
    },
    {
      key: "failed",
      label: "Failed",
      value: counts.failed,
      icon: counts.failed > 0 ? CircleAlert : undefined,
      className: counts.failed > 0 ? FAILED_FIGURE : PLAIN_FIGURE,
      testId: "loop-strategy-count-failed",
    },
    {
      key: "canceled",
      label: CANCELED.word,
      value: counts.canceledByStrategy,
      icon: CANCELED.icon,
      className: CALM_FIGURE,
      testId: "loop-strategy-count-canceled",
    },
  ];
  if (counts.active > 0) {
    figures.push({ key: "active", label: "Active", value: counts.active, className: PLAIN_FIGURE });
  }
  if (counts.pending > 0) {
    figures.push({
      key: "pending",
      label: "Pending",
      value: counts.pending,
      className: PLAIN_FIGURE,
    });
  }
  figures.push({
    key: "never-materialized",
    label: NEVER_MATERIALIZED.word,
    value: counts.neverMaterialized,
    icon: NEVER_MATERIALIZED.icon,
    className: CALM_FIGURE,
    testId: "loop-strategy-count-never-materialized",
  });
  return figures;
}

function microTrail(model: LoopStrategyProgressModel): string {
  const parts = [`fan_out ${model.nodeId}`];
  if (model.joinNodeId) parts.push(`join ${model.joinNodeId}`);
  parts.push(`completion_state ${model.completionState}`);
  return parts.join(" · ");
}

function hasKicker(model: LoopStrategyProgressModel): boolean {
  return (
    model.isPartial ||
    model.strategyLabel !== null ||
    model.triggerLane !== null ||
    model.winningLane !== null
  );
}

function StrategyKicker({ model }: { model: LoopStrategyProgressModel }) {
  if (!hasKicker(model)) return null;
  const StrategyGlyph = STRATEGY_GLYPH[model.strategyKind] ?? GitFork;
  const PartialGlyph = PARTIAL.icon;
  return (
    <div className="mb-2.5 flex flex-wrap items-center gap-1.5">
      {model.isPartial ? (
        <Pill data-testid="loop-strategy-partial" tone={PARTIAL.tone}>
          <PartialGlyph aria-hidden="true" />
          {`${PARTIAL.word} · ${model.coverageLabel}`}
        </Pill>
      ) : null}

      {model.strategyLabel === null ? null : (
        <Pill data-testid="loop-strategy-kind" mono size="xs">
          <StrategyGlyph aria-hidden="true" />
          {model.strategyLabel}
        </Pill>
      )}
      {model.triggerLane === null ? null : (
        <Pill data-testid="loop-strategy-trigger" mono size="xs">
          <Zap aria-hidden="true" />
          {`trigger · lane ${model.triggerLane}`}
        </Pill>
      )}
      {model.winningLane === null ? null : (
        <Pill data-testid="loop-strategy-winner" mono size="xs">
          <Flag aria-hidden="true" />
          {`winner · lane ${model.winningLane}`}
        </Pill>
      )}
    </div>
  );
}

function StrategyFigures({ counts }: { counts: LoopStrategyCounts }) {
  return (
    <dl className="mt-3 flex flex-wrap gap-x-5.5 gap-y-2">
      {strategyFigures(counts).map(figure => (
        <div className="flex flex-col gap-0.5" key={figure.key}>
          <dt>
            <Eyebrow className="text-faint">{figure.label}</Eyebrow>
          </dt>
          <dd
            className={cn(
              "inline-flex items-center gap-1 font-mono text-mono-id tabular-nums",
              figure.className
            )}
            data-testid={figure.testId}
          >
            {figure.icon ? <figure.icon aria-hidden="true" className="size-3 shrink-0" /> : null}
            {figure.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function EmptyCollectionNote() {
  const AbsenceGlyph = NEVER_MATERIALIZED.icon;
  return (
    <div className="mt-3 flex items-start gap-2.5" data-testid="loop-strategy-empty">
      <AbsenceGlyph aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
      <p className="max-w-[62ch] text-small-body leading-relaxed text-muted">
        The collection was empty, so no lanes opened and the strategy did not apply. The run
        continues.
      </p>
    </div>
  );
}

function StrategyBlock({ model }: { model: LoopStrategyProgressModel }) {
  const { counts } = model;
  const width = counts.opened + counts.neverMaterialized;
  const isEmpty = width === 0;
  return (
    <div
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-testid={`loop-strategy-block-${model.nodeId}`}
    >
      <div className="px-4.5 pt-3.5 pb-4">
        <StrategyKicker model={model} />
        <h3 className="text-ws-name font-medium tracking-tight text-fg-strong">
          {`Fan-out ${model.nodeId}`}
        </h3>
        <div className="mt-1.5 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-form-label text-muted">
          <span data-testid="loop-strategy-coverage">{`Covered ${model.coverageLabel}`}</span>
          {model.threshold ? (
            <span className="whitespace-nowrap text-subtle">{`${model.threshold} required`}</span>
          ) : null}
        </div>
        {isEmpty ? <EmptyCollectionNote /> : <StrategyFigures counts={counts} />}

        {model.isWide ? (
          <p className="mt-2.5 text-form-label text-subtle" data-testid="loop-strategy-aggregate">
            {`Aggregate across ${width} lanes.`}
          </p>
        ) : null}
        {model.missingAcceptable ? (
          <p className="mt-2 max-w-[62ch] text-small-body leading-relaxed text-muted">
            The author accepted missing lanes.
          </p>
        ) : null}
        <div className="mt-3 font-mono text-pill-group-badge text-faint">{microTrail(model)}</div>
      </div>
    </div>
  );
}

export interface LoopStrategyProgressProps {
  models: readonly LoopStrategyProgressModel[];
}

export function LoopStrategyProgress({ models }: LoopStrategyProgressProps) {
  if (models.length === 0) return null;
  return (
    <div className="flex flex-col gap-2" data-testid="loop-strategy-progress">
      {models.map(model => (
        <StrategyBlock key={model.nodeId} model={model} />
      ))}
    </div>
  );
}
