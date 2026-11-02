/** The agent's payoff in numbers: how much it did, when, and what it cost. */
import { useState } from "react";

import {
  DayAreaChart,
  DayStackedBars,
  Eyebrow,
  Panel,
  PillGroup,
  Section,
  StatusBreakdown,
} from "@compozy/ui";

import {
  HOME_OUTCOMES_LEGEND_CLASS,
  HOME_USAGE_HEADLINE_CLASS,
} from "@/systems/dashboard/components/home-zone-shape";
import { HomePulseHeatmap } from "@/systems/dashboard/components/home-pulse-heatmap";
import { ProviderUsageRow } from "@/systems/provider-usage/components/provider-usage-row";
import { toProviderUsageRow } from "@/systems/provider-usage/lib/view-model";
import {
  PROVIDER_USAGE_FIXTURE_NOW,
  claudeOkTarget,
} from "@/systems/provider-usage/mocks/fixtures";

import type { ProfileData } from "./profile-fixtures";

const SERIES = [
  { key: "quiet", label: "Nothing new", fill: "var(--color-bar-fill)" },
  { key: "reported", label: "Told you something", fill: "var(--color-viz-bar)" },
  { key: "needed", label: "Needed you", fill: "var(--color-fg)" },
];

const WINDOWS = [
  { value: "7", label: "7 days" },
  { value: "30", label: "30 days" },
];

/** Lets the first and last date labels sit past the plot's edge. */
const CHART_CLASS = "min-w-0 [&_.recharts-surface]:overflow-visible";

/** The dashboard's chart headline: the number, then what it means on one line. */
function Headline({ value, note }: { value: string; note: string }) {
  return (
    <div className={HOME_USAGE_HEADLINE_CLASS}>
      <span className="text-kpi-value font-semibold text-fg-strong tabular-nums">{value}</span>
      <span className="text-small-body text-subtle">{note}</span>
    </div>
  );
}

function Legend() {
  return (
    <div className={HOME_OUTCOMES_LEGEND_CLASS}>
      {SERIES.map(entry => (
        <span className="inline-flex items-center gap-1.5" key={entry.key}>
          <span
            aria-hidden="true"
            className="size-2 rounded-xs"
            style={{ background: entry.fill }}
          />
          {entry.label}
        </span>
      ))}
    </div>
  );
}

export function ActivitySection({ data }: { data: ProfileData }) {
  const [days, setDays] = useState("30");
  const window = Number(days);
  const onDemand = data.agent.kind === "on_demand";
  const { unit, sub, value } = data.headline;
  return (
    <>
      <Section
        label="How it's been going"
        right={<PillGroup items={WINDOWS} onChange={setDays} size="sm" value={days} />}
      >
        <Panel bodyClassName="flex flex-col gap-3">
          <Headline note={unit ? `${unit} · ${sub}` : sub} value={value} />
          <div className={CHART_CLASS}>
            <DayStackedBars
              ariaLabel={`${data.agent.label}'s activity, day by day`}
              data={data.days.slice(-window).map(entry => ({ ...entry }))}
              height={132}
              series={SERIES}
              withTooltip
            />
          </div>
          {onDemand ? null : <Legend />}
        </Panel>
      </Section>
      <Section
        label="What it cost"
        right={<span className="text-eyebrow text-muted-foreground">Last {days} days</span>}
      >
        <Panel bodyClassName="flex flex-col gap-3">
          <Headline
            note={
              data.usage.cost === "Included"
                ? `${data.usage.tokens}, in your plan`
                : `${data.usage.tokens}, estimated`
            }
            value={data.usage.cost}
          />
          <div className={CHART_CLASS}>
            <DayAreaChart
              ariaLabel={`${data.agent.label}'s usage, day by day`}
              data={data.usage.days.slice(-window)}
              formatValue={point => `${Math.round(point / 1000)}K tokens`}
              height={132}
              withTooltip
            />
          </div>
          <div className="-mx-5 flex flex-col gap-2.5 border-t border-line-soft px-5 pt-3.5">
            <Eyebrow className="text-subtle">By project</Eyebrow>
            <StatusBreakdown
              items={data.usageByProject.map(entry => ({
                id: entry.project,
                label: entry.project,
                value: entry.share,
                formattedValue: entry.amount,
              }))}
            />
          </div>
        </Panel>
      </Section>
    </>
  );
}

export function RhythmSection({ data }: { data: ProfileData }) {
  const busiest = data.pulse.reduce<ProfileData["pulse"][number] | null>(
    (best, bucket) => (bucket.events > (best?.events ?? 0) ? bucket : null) ?? best,
    null
  );
  return (
    <HomePulseHeatmap
      label="When it's busy"
      pulse={{ buckets: [...data.pulse], busiest, longest_session: null, window_days: 14 }}
    />
  );
}

/** The row keeps its own 8px inset, so the body adds what lands it on the panel's 20px edge. */
export function PlanCard() {
  return (
    <Section label="Your plan">
      <Panel bodyClassName="p-3">
        <ProviderUsageRow
          isMacHost
          now={PROVIDER_USAGE_FIXTURE_NOW}
          row={toProviderUsageRow(claudeOkTarget())}
        />
      </Panel>
    </Section>
  );
}
