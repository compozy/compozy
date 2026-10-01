import { Link } from "@tanstack/react-router";

import { Metric, MetricGrid, Sparkline } from "@compozy/ui";

import { formatHomeTokens } from "../lib/home-formatters";
import type { HomeOverview } from "../types";

export interface HomeKpiStripProps {
  overview: HomeOverview;
  workingNowTotal: number;
  workingNowDetail: string;
}

function attentionDetail(byKind: Record<string, number>): string {
  const parts: string[] = [];
  if (byKind.approval) {
    parts.push(`${byKind.approval} approval${byKind.approval === 1 ? "" : "s"}`);
  }
  if (byKind.needs_input) {
    parts.push(`${byKind.needs_input} input`);
  }
  if (byKind.failure) {
    parts.push(`${byKind.failure} failure${byKind.failure === 1 ? "" : "s"}`);
  }
  return parts.length > 0 ? parts.join(" · ") : "nothing waiting";
}

function usageCostDetail(overview: HomeOverview): string {
  const cost = overview.usage.estimated_cost;
  if (cost === undefined || cost === null) {
    return overview.usage.cost_status === "included" ? "cost included in plan" : "cost unavailable";
  }
  return `≈ ${cost.toFixed(2)} ${overview.usage.cost_currency ?? ""} estimated`.trimEnd();
}

// A linked tile lifts its own card on hover — the wrapping link has no fill of its own.
const KPI_LINK_METRIC = "transition-colors duration-base group-hover/kpi:bg-surface-2";

/**
 * Zone 2 — the four at-a-glance counters. Every tile deep-links into the
 * surface that owns the number; the usage tile carries the 30d spark.
 */
export function HomeKpiStrip({ overview, workingNowTotal, workingNowDetail }: HomeKpiStripProps) {
  const usageValues = overview.usage.days.map(day => day.tokens);

  return (
    <MetricGrid columns={4} data-slot="home-kpi-strip">
      <Link
        className="group/kpi min-w-0 rounded-lg focus-visible:shadow-focus-ring focus-visible:outline-none"
        search={{ mode: "dashboard" }}
        to="/tasks"
      >
        <Metric
          className={KPI_LINK_METRIC}
          label="Working now"
          labelCase="eyebrow"
          subtext={workingNowDetail}
          value={workingNowTotal}
        />
      </Link>
      <Link
        className="group/kpi min-w-0 rounded-lg focus-visible:shadow-focus-ring focus-visible:outline-none"
        search={{ mode: "inbox" }}
        to="/tasks"
      >
        <Metric
          className={KPI_LINK_METRIC}
          label="Needs you"
          labelCase="eyebrow"
          subtext={attentionDetail(overview.attention.by_kind ?? {})}
          value={overview.attention.total}
        />
      </Link>
      <Link
        className="group/kpi min-w-0 rounded-lg focus-visible:shadow-focus-ring focus-visible:outline-none"
        search={{ mode: "dashboard" }}
        to="/tasks"
      >
        <Metric
          className={KPI_LINK_METRIC}
          label="Completed today"
          labelCase="eyebrow"
          subtext={`${overview.today.runs_completed} runs, ${overview.today.tasks_closed} tasks finished`}
          value={overview.today.runs_completed + overview.today.tasks_closed}
        />
      </Link>
      <div className="min-w-0 rounded-lg" data-slot="home-kpi-usage">
        <Metric
          label={`Usage, last ${overview.usage.window_days} days`}
          labelCase="eyebrow"
          subtext={usageCostDetail(overview)}
          trailing={
            usageValues.some(value => value > 0) ? (
              <Sparkline
                ariaLabel={`Tokens per day over the last ${overview.usage.window_days} days`}
                className="w-16"
                height={22}
                values={usageValues}
              />
            ) : undefined
          }
          value={formatHomeTokens(overview.usage.total_tokens)}
        />
      </div>
    </MetricGrid>
  );
}
