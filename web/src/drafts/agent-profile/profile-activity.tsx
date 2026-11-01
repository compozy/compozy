/** The agent's payoff in numbers: how much it did, when, and what it cost. */
import { useState, type ReactNode } from "react";

import {
  DayAreaChart,
  DayStackedBars,
  Panel,
  PillGroup,
  Section,
  StatusBreakdown,
} from "@compozy/ui";

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

function Figure({
  value,
  unit,
  sub,
  label,
}: {
  label: string;
  value: string;
  unit?: string;
  sub: string;
}) {
  return (
    <div className="flex min-w-0 flex-col justify-end gap-1">
      <span className="mb-auto text-small-body text-muted">{label}</span>
      <span className="flex items-baseline gap-1.5">
        <span className="text-display-2xl font-semibold text-fg-strong tabular-nums">{value}</span>
        {unit ? <span className="text-small-body text-muted">{unit}</span> : null}
      </span>
      <span className="text-eyebrow text-subtle">{sub}</span>
    </div>
  );
}

function ChartCard({
  figure,
  chart,
  foot,
}: {
  figure: ReactNode;
  chart: ReactNode;
  foot?: ReactNode;
}) {
  return (
    <Panel foot={foot}>
      <div className="grid min-h-36 grid-cols-1 gap-5 @lg:grid-cols-[minmax(0,12rem)_minmax(0,1fr)]">
        {figure}
        <div className="min-w-0 [&_.recharts-surface]:overflow-visible">{chart}</div>
      </div>
    </Panel>
  );
}

function Legend() {
  return (
    <div className="flex flex-wrap items-center gap-4 text-eyebrow text-muted">
      {SERIES.map(entry => (
        <span className="flex items-center gap-1.5" key={entry.key}>
          <span
            aria-hidden="true"
            className="size-2 rounded-xxs"
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
  const onDemand = data.agent.kind === "ephemeral";
  return (
    <Section
      label="How it's been going"
      right={<PillGroup items={WINDOWS} onChange={setDays} size="sm" value={days} />}
    >
      <div className="flex flex-col gap-3">
        <ChartCard
          chart={
            <DayStackedBars
              ariaLabel={`${data.agent.label}'s activity, day by day`}
              data={data.days.slice(-window).map(entry => ({ ...entry }))}
              height={132}
              series={SERIES}
              withTooltip
            />
          }
          figure={
            <Figure
              label={onDemand ? "Sessions" : "Reports"}
              sub={data.headline.sub}
              unit={data.headline.unit}
              value={data.headline.value}
            />
          }
          foot={onDemand ? undefined : <Legend />}
        />
        <ChartCard
          chart={
            <DayAreaChart
              ariaLabel={`${data.agent.label}'s usage, day by day`}
              data={data.usage.days.slice(-window)}
              formatValue={value => `${Math.round(value / 1000)}K tokens`}
              height={132}
              withTooltip
            />
          }
          figure={
            <Figure
              label="What it cost"
              sub={
                data.usage.cost === "Included"
                  ? `${data.usage.tokens}, in your plan`
                  : `${data.usage.tokens}, estimated`
              }
              value={data.usage.cost}
            />
          }
          foot={
            <StatusBreakdown
              className="flex-1"
              items={data.usageByProject.map(entry => ({
                id: entry.project,
                label: entry.project,
                value: entry.share,
                formattedValue: entry.amount,
              }))}
            />
          }
        />
      </div>
    </Section>
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

export function PlanCard() {
  return (
    <Panel bodyClassName="p-0" title="Your plan">
      <ProviderUsageRow
        isMacHost
        now={PROVIDER_USAGE_FIXTURE_NOW}
        row={toProviderUsageRow(claudeOkTarget())}
      />
    </Panel>
  );
}
