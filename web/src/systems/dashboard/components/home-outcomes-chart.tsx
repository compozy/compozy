import { DayStackedBars, Empty, Panel, Section } from "@agh/ui";

import type { HomeOverview } from "../types";

export interface HomeOutcomesChartProps {
  outcomes: HomeOverview["outcomes"];
}

const OUTCOME_SERIES = [
  { key: "completed", fill: "var(--color-success)", label: "completed" },
  { key: "canceled", fill: "var(--color-viz-other)", label: "canceled" },
  { key: "failed", fill: "var(--color-danger)", label: "failed" },
] as const;

/**
 * Zone 4a — daily run outcomes for the last 14 days. Status colors are the
 * data here (the one sanctioned signal-color use in home charts); canceled
 * stays neutral ink and separates the two hues for color-blind readers.
 */
export function HomeOutcomesChart({ outcomes }: HomeOutcomesChartProps) {
  const total = outcomes.completed + outcomes.failed + outcomes.canceled;
  const hasData = outcomes.days.some(
    day => day.completed > 0 || day.failed > 0 || day.canceled > 0
  );

  return (
    <Section
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      data-slot="home-outcomes"
      label="Outcomes · last 14 days"
    >
      <Panel bodyClassName="flex flex-1 flex-col gap-3" className="flex-1">
        {hasData ? (
          <>
            <div className="flex items-baseline gap-2.5">
              <span
                className="text-kpi-value text-fg-strong tabular-nums"
                style={{ fontWeight: "var(--font-weight-display)" }}
              >
                {Math.round(outcomes.success_pct)}%
              </span>
              <span className="text-small-body text-subtle">
                success — {outcomes.completed} of {total} runs completed
              </span>
            </div>
            <DayStackedBars
              ariaLabel={`Stacked daily bars of run outcomes for the last 14 days: ${outcomes.completed} completed, ${outcomes.failed} failed, ${outcomes.canceled} canceled`}
              className="min-h-[120px] flex-1"
              data={outcomes.days}
              series={OUTCOME_SERIES}
            />
            <div className="flex flex-wrap items-center gap-4 text-small-body text-muted">
              <LegendEntry
                color="var(--color-success)"
                count={outcomes.completed}
                label="Completed"
              />
              <LegendEntry
                color="var(--color-viz-other)"
                count={outcomes.canceled}
                label="Canceled"
              />
              <LegendEntry color="var(--color-danger)" count={outcomes.failed} label="Failed" />
            </div>
            <p className="text-micro leading-relaxed text-faint">
              Each bar counts task runs that ended that day.
            </p>
          </>
        ) : (
          <Empty
            description="Run outcomes chart here once task runs finish."
            title="No runs ended in the last 14 days"
          />
        )}
      </Panel>
    </Section>
  );
}

function LegendEntry({ color, count, label }: { color: string; count: number; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span aria-hidden="true" className="size-2 rounded-xs" style={{ background: color }} />
      {label} <span className="font-mono text-micro tabular-nums text-subtle">{count}</span>
    </span>
  );
}
