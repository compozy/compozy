import { DayAreaChart, Empty, Eyebrow, Panel, PillGroup, Section } from "@agh/ui";

import {
  HOME_USAGE_WINDOWS,
  normalizeHomeUsageWindow,
  type HomeOverview,
  type HomeUsageWindow,
} from "../types";
import { formatHomeTokens } from "../lib/home-formatters";

export interface HomeUsageChartProps {
  usage: HomeOverview["usage"];
  window: HomeUsageWindow;
  onWindowChange: (window: HomeUsageWindow) => void;
}

function usageFigure(usage: HomeOverview["usage"]): string {
  const cost = usage.estimated_cost;
  if (cost === undefined || cost === null) {
    if (usage.cost_status === "included") {
      return `cost included · ${usage.days.length} days`;
    }
    return `${usage.days.length} days`;
  }
  const head = ["≈", cost.toFixed(2), usage.cost_currency, "estimated"]
    .filter(part => part !== undefined && part !== null && part !== "")
    .join(" ");
  return `${head} · ${usage.days.length} days`;
}

/**
 * Zone 4b — tokens per day for the persisted 7/30/90d window, painted with
 * neutral data ink, plus the 30d per-agent share bar. Cost lines render only
 * from truthful provenance; the retention footnote appears when the window
 * outruns pruned history.
 */
export function HomeUsageChart({ usage, window, onWindowChange }: HomeUsageChartProps) {
  const hasData = usage.days.some(day => day.tokens > 0);
  const costPerToken =
    usage.estimated_cost !== undefined && usage.estimated_cost !== null && usage.total_tokens > 0
      ? usage.estimated_cost / usage.total_tokens
      : null;

  const footnotes: string[] = [];
  if (usage.truncated) {
    footnotes.push(
      `Showing ${usage.retention_days} of ${usage.window_days} days — older usage is pruned by the retention window.`
    );
  }
  if (usage.estimated_cost !== undefined && usage.estimated_cost !== null) {
    footnotes.push(
      "Cost is estimated from model-catalog pricing and may diverge from provider billing."
    );
  }

  return (
    <Section
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      data-slot="home-usage"
      label="Usage & cost"
      right={
        <PillGroup
          items={HOME_USAGE_WINDOWS.map(value => ({ value: String(value), label: `${value}d` }))}
          onChange={next => onWindowChange(normalizeHomeUsageWindow(Number(next)))}
          size="sm"
          value={String(window)}
        />
      }
    >
      <Panel bodyClassName="flex flex-1 flex-col gap-3" className="flex-1">
        <div className="flex items-baseline gap-2.5">
          <span
            className="text-kpi-value text-fg-strong tabular-nums"
            style={{ fontWeight: "var(--font-weight-display)" }}
          >
            {formatHomeTokens(usage.total_tokens)}
          </span>
          <span className="text-small-body text-subtle">{usageFigure(usage)}</span>
        </div>
        {hasData ? (
          <DayAreaChart
            ariaLabel={`Tokens per day over the selected ${usage.window_days}-day window`}
            className="min-h-[100px] flex-1"
            data={usage.days.map(day => ({ date: day.date, value: day.tokens }))}
            formatValue={value =>
              costPerToken === null
                ? `${formatHomeTokens(value)} tokens`
                : `${formatHomeTokens(value)} tokens · ≈ ${(value * costPerToken).toFixed(2)}`
            }
          />
        ) : (
          <Empty
            description="Daily token usage charts here as agents work."
            title="No usage recorded in this window"
          />
        )}
        {footnotes.length > 0 ? (
          <p className="text-micro leading-relaxed text-faint">{footnotes.join(" ")}</p>
        ) : null}
        <HomeAgentShare share={usage.agent_share} />
      </Panel>
    </Section>
  );
}

const SHARE_RAMP = [
  "var(--color-viz-line)",
  "var(--color-viz-bar)",
  "var(--color-viz-other)",
  "var(--color-bar-fill)",
] as const;

function HomeAgentShare({ share }: { share: HomeOverview["usage"]["agent_share"] }) {
  if (share.length === 0) {
    return null;
  }
  const label = share
    .map(entry => `${entry.agent_name} ${Math.round(entry.fraction * 100)}%`)
    .join(", ");

  return (
    <div
      className="-mx-5 mt-auto border-t border-line-soft px-5 pt-3.5"
      data-slot="home-agent-share"
    >
      <div className="mb-2.5 flex items-center justify-between gap-3">
        <Eyebrow className="text-subtle">Per-agent share</Eyebrow>
        <span className="text-micro text-faint">last 30 days</span>
      </div>
      <div
        aria-label={`Token share by agent: ${label}`}
        className="flex h-2.5 gap-0.5 overflow-hidden rounded-xs"
        role="img"
      >
        {share.map((entry, index) => (
          <span
            key={entry.agent_name}
            style={{
              background: SHARE_RAMP[Math.min(index, SHARE_RAMP.length - 1)],
              width: `${Math.max(1, entry.fraction * 100)}%`,
            }}
          />
        ))}
      </div>
      <div className="mt-2.5 flex flex-wrap items-center gap-4 text-small-body text-muted">
        {share.map((entry, index) => (
          <span className="inline-flex items-center gap-1.5" key={entry.agent_name}>
            <span
              aria-hidden="true"
              className="size-2 rounded-xs"
              style={{ background: SHARE_RAMP[Math.min(index, SHARE_RAMP.length - 1)] }}
            />
            {entry.agent_name}{" "}
            <span className="font-mono text-micro tabular-nums text-subtle">
              {Math.round(entry.fraction * 100)}% · {formatHomeTokens(entry.tokens)}
            </span>
          </span>
        ))}
      </div>
    </div>
  );
}
