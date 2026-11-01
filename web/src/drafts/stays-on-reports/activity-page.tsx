/** S14: the Dashboard's Activity page, built from the Dashboard's own zones. */
import { CircleHelp, MoonStar } from "lucide-react";
import { Fragment, useState } from "react";

import {
  DayStackedBars,
  Empty,
  MetricGrid,
  Metric,
  PAGE_CONTENT_GUTTER,
  Panel,
  Pill,
  PillGroup,
  SearchInput,
  Section,
  Sparkline,
  StatusBreakdown,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  cn,
  type PillTone,
} from "@compozy/ui";

import { HomePageMeta } from "@/systems/dashboard/components/home-page-meta";
import { HomePulseHeatmap } from "@/systems/dashboard/components/home-pulse-heatmap";
import {
  HOME_BODY_CLASS,
  HOME_OUTCOMES_LEGEND_CLASS,
  HOME_ZONE_PAIR_CLASS,
  HOME_ZONE_STACK_CLASS,
} from "@/systems/dashboard/components/home-zone-shape";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { DraftShell } from "../stays-on-agents-shared/draft-shell";
import {
  AGENT_SHARE,
  DAYS,
  ENTRIES,
  LEGACY,
  PULSE,
  STARTER_LABEL,
  STATS,
  UNATTENDED,
  type ActivityEntry,
  type Result,
} from "./activity-fixtures";

type Filter = "all" | "unattended";

const RESULT: Record<Result, { label: string; tone: PillTone }> = {
  allowed: { label: "Allowed", tone: "neutral" },
  declined: { label: "Declined", tone: "warning" },
  expired: { label: "No answer", tone: "warning" },
};

const SERIES = [
  { key: "allowed", fill: "var(--color-viz-bar)", label: "Allowed" },
  { key: "declined", fill: "var(--color-warning)", label: "Declined" },
  { key: "expired", fill: "var(--color-danger)", label: "No answer" },
] as const;

function trend(key: Result) {
  return DAYS.map(day => day[key]);
}

function Trend({ label, values }: { label: string; values: readonly number[] }) {
  return <Sparkline ariaLabel={label} className="w-16" height={22} values={values} />;
}

function Stats({ filter }: { filter: Filter }) {
  const stats = STATS[filter];
  const share = Math.round((stats.allowed / stats.total) * 100);
  return (
    <MetricGrid columns={4}>
      <Metric
        label={filter === "all" ? "Actions" : "Ran without you"}
        labelCase="eyebrow"
        subtext="last 14 days"
        trailing={
          <Trend
            label="Actions per day over the last 14 days"
            values={DAYS.map(day => day.allowed + day.declined + day.expired)}
          />
        }
        value={stats.total.toLocaleString("en-US")}
      />
      <Metric
        label="Allowed"
        labelCase="eyebrow"
        subtext={`${share}% went straight through`}
        trailing={<Trend label="Allowed per day" values={trend("allowed")} />}
        value={stats.allowed.toLocaleString("en-US")}
      />
      <Metric
        label="Declined"
        labelCase="eyebrow"
        subtext="a rule said no"
        trailing={<Trend label="Declined per day" values={trend("declined")} />}
        value={stats.declined}
      />
      <Metric
        label="No answer"
        labelCase="eyebrow"
        subtext="waited, then stopped"
        tone={stats.expired > 0 ? "warning" : "default"}
        trailing={<Trend label="No answer per day" values={trend("expired")} />}
        value={stats.expired}
      />
    </MetricGrid>
  );
}

function Outcomes() {
  return (
    <Section label="What happened" right={<span className="text-eyebrow text-muted">By day</span>}>
      <Panel bodyClassName="flex flex-col gap-3">
        <DayStackedBars
          ariaLabel="Actions per day by outcome over the last 14 days"
          className="min-h-44"
          data={DAYS}
          series={SERIES}
        />
        <div className={HOME_OUTCOMES_LEGEND_CLASS}>
          {SERIES.map(item => (
            <span className="inline-flex items-center gap-1.5" key={item.key}>
              <span
                aria-hidden="true"
                className="size-2 rounded-xs"
                style={{ background: item.fill }}
              />
              {item.label}
            </span>
          ))}
        </div>
      </Panel>
    </Section>
  );
}

function WhoStarted({ filter }: { filter: Filter }) {
  return (
    <Section label="Who started it">
      <Panel>
        <StatusBreakdown
          items={STATS[filter].starters.map(item => ({
            id: item.key,
            label: item.label,
            value: item.value,
          }))}
        />
      </Panel>
    </Section>
  );
}

function WhichAgents() {
  return (
    <Section label="Which agents">
      <Panel>
        <StatusBreakdown
          items={AGENT_SHARE.map(({ agent, value }) => ({
            id: agent.name,
            label: agent.label,
            swatch: (
              <AgentAvatar
                appearance={agent.appearance}
                label={agent.label}
                quietIdle
                size={18}
                surfaceClassName="ring-card"
              />
            ),
            value,
          }))}
        />
      </Panel>
    </Section>
  );
}

function StartedBy({ entry }: { entry: ActivityEntry }) {
  if (entry.starter === "unknown") {
    return (
      <span className="inline-flex items-center gap-1.5 text-subtle">
        <CircleHelp aria-hidden="true" className="size-3.5" />
        Unknown
      </span>
    );
  }
  return (
    <span className="flex min-w-0 flex-col">
      <span className="truncate">{STARTER_LABEL[entry.starter]}</span>
      {entry.source ? (
        <span className="truncate text-eyebrow text-subtle">{entry.source}</span>
      ) : null}
    </span>
  );
}

function Row({ entry }: { entry: ActivityEntry }) {
  const Icon = entry.icon;
  const result = RESULT[entry.result];
  return (
    <TableRow>
      <TableCell className="w-16 font-mono text-mono-id text-subtle">{entry.when}</TableCell>
      <TableCell>
        <span className="flex min-w-0 items-center gap-2.5">
          <Icon aria-hidden="true" className="size-4 shrink-0 text-subtle" />
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body text-fg">{entry.action}</span>
            <span className="truncate font-mono text-mono-id text-subtle">{entry.detail}</span>
          </span>
        </span>
      </TableCell>
      <TableCell>
        <span className="inline-flex items-center gap-2">
          <AgentAvatar
            appearance={entry.who.appearance}
            label={entry.who.label}
            quietIdle
            size={22}
          />
          {entry.who.label}
        </span>
      </TableCell>
      <TableCell>
        <StartedBy entry={entry} />
      </TableCell>
      <TableCell className="text-right">
        <Pill size="sm" tone={result.tone}>
          {result.label}
        </Pill>
      </TableCell>
    </TableRow>
  );
}

function Log({ entries }: { entries: readonly ActivityEntry[] }) {
  const byDay = new Map<string, ActivityEntry[]>();
  for (const entry of entries) {
    byDay.set(entry.day, [...(byDay.get(entry.day) ?? []), entry]);
  }
  return (
    <Panel bodyClassName="p-0">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>When</TableHead>
            <TableHead>What</TableHead>
            <TableHead>Agent</TableHead>
            <TableHead>Started by</TableHead>
            <TableHead className="text-right">Result</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {[...byDay].map(([day, members]) => (
            <Fragment key={day}>
              <TableRow>
                <TableCell className="bg-surface-2/40 py-1.5 text-eyebrow text-muted" colSpan={5}>
                  {day}
                </TableCell>
              </TableRow>
              {members.map(entry => (
                <Row entry={entry} key={entry.id} />
              ))}
            </Fragment>
          ))}
        </TableBody>
      </Table>
    </Panel>
  );
}

export function ActivityPage({
  initial,
  legacy = false,
  empty = false,
}: {
  initial: Filter;
  legacy?: boolean;
  empty?: boolean;
}) {
  const [filter, setFilter] = useState<Filter>(initial);
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const rows = (empty ? [] : [...ENTRIES, ...(legacy ? [LEGACY] : [])]).filter(
    entry =>
      (filter === "all" || UNATTENDED.has(entry.starter)) &&
      `${entry.action} ${entry.detail} ${entry.who.label}`.toLowerCase().includes(needle)
  );
  const toolbar = (
    <>
      <PillGroup
        items={[
          { value: "all", label: "Everything" },
          { value: "unattended", label: "Nobody watching" },
        ]}
        onChange={setFilter}
        size="sm"
        value={filter}
      />
      <SearchInput
        aria-label="Search activity"
        containerClassName="ml-auto w-64"
        onChange={setQuery}
        placeholder="Search activity…"
        value={query}
      />
    </>
  );
  return (
    <DraftShell app="dashboard" sessionBadge={0} title="Activity" topbar={{ toolbar }}>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className={cn(PAGE_CONTENT_GUTTER, HOME_BODY_CLASS)}>
          <HomePageMeta workspaceName="compozy" />
          {empty ? (
            <Panel>
              <Empty
                description="When a check-in, automation or extension does something on its own, it shows up here."
                icon={MoonStar}
                title="Nothing ran on its own yet"
              />
            </Panel>
          ) : (
            <div className={HOME_ZONE_STACK_CLASS}>
              <h2 className="text-detail-h1 font-medium text-fg-strong">
                {STATS[filter].sentence}
              </h2>
              <Stats filter={filter} />
              <HomePulseHeatmap pulse={PULSE} />
              <div className={HOME_ZONE_PAIR_CLASS}>
                <Outcomes />
                <div className="flex flex-col gap-5">
                  <WhoStarted filter={filter} />
                  <WhichAgents />
                </div>
              </div>
              <Section count={rows.length} label="Everything it did">
                {rows.length > 0 ? (
                  <Log entries={rows} />
                ) : (
                  <Panel>
                    <Empty description="Try a different word." title="Nothing matches that" />
                  </Panel>
                )}
              </Section>
            </div>
          )}
        </div>
      </div>
    </DraftShell>
  );
}
