/** The agent's payoff in numbers: how much it did, when, and what it cost. */
import { Moon } from "@compozy/ui/icons";
import { useState } from "react";

import {
  DayStackedBars,
  Empty,
  Eyebrow,
  Panel,
  PillGroup,
  Section,
  StatusBreakdown,
} from "@compozy/ui";

import {
  HOME_OUTCOMES_LEGEND_CLASS,
  HOME_USAGE_HEADLINE_CLASS,
  HomePulseHeatmap,
} from "@/systems/dashboard";
import { useProfileReadScope } from "@/systems/profiles";
import { ProviderUsageRow, useProviderUsage, useUsageClock } from "@/systems/provider-usage";

import { agentActivityView, agentCostView } from "../../lib/agent-profile";
import type { AgentActivityPayload } from "../../types";

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

export function AgentFirstDay({ name, staysOn }: { name: string; staysOn: boolean }) {
  return (
    <Section label="How it's been going">
      <Panel bodyClassName="px-6 py-14">
        <Empty
          description={
            staysOn
              ? "Add it to a project and everything it does, finds and costs will show up here."
              : "Start a session and everything it does, finds and costs will show up here."
          }
          fill={false}
          icon={Moon}
          title={`${name} hasn't started yet`}
        />
      </Panel>
    </Section>
  );
}

/** No route answers yet: says so, instead of claiming the agent did nothing. */
export function AgentActivityUnavailable() {
  return (
    <Section label="How it's been going">
      <Panel bodyClassName="px-6 py-10">
        <Empty
          description="This version of CompozyOS doesn't report activity for agents yet."
          fill={false}
          icon={Moon}
          size="compact"
          title="Activity isn't available yet"
        />
      </Panel>
    </Section>
  );
}

export function AgentActivitySection({
  activity,
  label,
  onDemand,
}: {
  activity: AgentActivityPayload;
  label: string;
  onDemand: boolean;
}) {
  const [days, setDays] = useState("30");
  const view = agentActivityView(activity, Number(days));
  const cost = agentCostView(activity);
  return (
    <>
      <Section
        label="How it's been going"
        right={<PillGroup items={WINDOWS} onChange={setDays} size="sm" value={days} />}
      >
        <Panel bodyClassName="flex flex-col gap-3">
          <Headline
            note={`${view.reports === 1 ? "report" : "reports"} · ${view.perDay} a day`}
            value={view.reports.toLocaleString("en-US")}
          />
          <div className={CHART_CLASS}>
            <DayStackedBars
              ariaLabel={`${label}'s activity, day by day`}
              data={view.days}
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
        right={<span className="text-eyebrow text-muted-foreground">Last 30 days</span>}
      >
        <Panel bodyClassName="flex flex-col gap-3">
          <Headline note={`${cost.tokens}, estimated`} value={cost.cost} />
          {cost.projects.length > 0 ? (
            <div className="-mx-5 flex flex-col gap-2.5 border-t border-line-soft px-5 pt-3.5">
              <Eyebrow className="text-subtle">By project</Eyebrow>
              <StatusBreakdown items={cost.projects} />
            </div>
          ) : null}
        </Panel>
      </Section>
    </>
  );
}

export function AgentRhythmSection({ activity }: { activity: AgentActivityPayload }) {
  const busiest = activity.busy_by_hour.reduce<AgentActivityPayload["busy_by_hour"][number] | null>(
    (best, bucket) => (bucket.events > (best?.events ?? 0) ? bucket : best),
    null
  );
  if (activity.busy_by_hour.length === 0) return null;
  return (
    <HomePulseHeatmap
      label="When it's busy"
      pulse={{
        buckets: [...activity.busy_by_hour],
        busiest,
        longest_session: null,
        window_days: 14,
      }}
    />
  );
}

/** The agent's subscription, when the operator's usage reads know about its provider. */
export function AgentPlanCard({ provider }: { provider: string | null }) {
  const { destination } = useProfileReadScope();
  const usage = useProviderUsage(destination, { enabled: Boolean(provider) });
  const now = useUsageClock();
  const row = usage.viewModel?.rows.find(entry => entry.target.provider_id === provider);
  if (!row) return null;
  return (
    <Section label="Your plan">
      {/* The row keeps its own 8px inset, so the body adds what lands it on the panel's 20px edge. */}
      <Panel bodyClassName="p-3">
        <ProviderUsageRow
          isMacHost={usage.viewModel?.keychainSupported ?? false}
          now={now}
          row={row}
        />
      </Panel>
    </Section>
  );
}
