/** S14: the Dashboard's Activity page, built from the Dashboard's own zones. */
import {
  CircleHelp,
  FileEdit,
  FilePlus2,
  FolderSearch,
  Globe,
  MoonStar,
  SquareTerminal,
  type LucideIcon,
} from "@compozy/ui/icons";
import { Fragment, type ReactNode } from "react";

import {
  DayStackedBars,
  Empty,
  Metric,
  MetricGrid,
  PAGE_CONTENT_GUTTER,
  Panel,
  Pill,
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

import {
  STARTER_LABEL,
  type ActivityFilter,
  type ActivityModel,
  type ActivityResult,
  type ActivityRow,
} from "../../lib/activity-view";
import { HomePageMeta } from "../home-page-meta";
import { HomePulseHeatmap } from "../home-pulse-heatmap";
import {
  HOME_BODY_CLASS,
  HOME_OUTCOMES_LEGEND_CLASS,
  HOME_ZONE_PAIR_CLASS,
  HOME_ZONE_STACK_CLASS,
} from "../home-zone-shape";

const RESULT: Record<ActivityResult, { label: string; tone: PillTone }> = {
  allowed: { label: "Allowed", tone: "neutral" },
  declined: { label: "Declined", tone: "warning" },
  expired: { label: "No answer", tone: "warning" },
};

const SERIES = [
  { key: "allowed", fill: "var(--color-viz-bar)", label: "Allowed" },
  { key: "declined", fill: "var(--color-warning)", label: "Declined" },
  { key: "expired", fill: "var(--color-destructive)", label: "No answer" },
] as const;

const ACTION_ICON: Record<string, LucideIcon> = {
  "Ran a command": SquareTerminal,
  "Wanted to run a command": SquareTerminal,
  "Edited a file": FileEdit,
  "Wanted to edit a file": FileEdit,
  "Wrote a file": FilePlus2,
  "Wanted to write a file": FilePlus2,
  "Read a file": FolderSearch,
  "Wanted to read a file": FolderSearch,
  "Looked something up": Globe,
  "Wanted to look something up": Globe,
};

export interface ActivityAgentMark {
  label: string;
  avatar: ReactNode;
}

function Trend({ label, values }: { label: string; values: readonly number[] }) {
  return <Sparkline ariaLabel={label} className="w-16" height={22} values={values} />;
}

function Stats({ filter, model }: { filter: ActivityFilter; model: ActivityModel }) {
  const share = model.total > 0 ? Math.round((model.allowed / model.total) * 100) : 0;
  const series = (key: ActivityResult) => model.days.map(day => day[key]);
  return (
    <MetricGrid columns={4}>
      <Metric
        label={filter === "all" ? "Actions" : "Ran without you"}
        labelCase="eyebrow"
        subtext="last 14 days"
        trailing={
          <Trend
            label="Actions per day over the last 14 days"
            values={model.days.map(day => day.allowed + day.declined + day.expired)}
          />
        }
        value={model.total.toLocaleString("en-US")}
      />
      <Metric
        label="Allowed"
        labelCase="eyebrow"
        subtext={`${share}% went straight through`}
        trailing={<Trend label="Allowed per day" values={series("allowed")} />}
        value={model.allowed.toLocaleString("en-US")}
      />
      <Metric
        label="Declined"
        labelCase="eyebrow"
        subtext="a rule said no"
        trailing={<Trend label="Declined per day" values={series("declined")} />}
        value={model.declined}
      />
      <Metric
        label="No answer"
        labelCase="eyebrow"
        subtext="waited, then stopped"
        tone={model.expired > 0 ? "warning" : "default"}
        trailing={<Trend label="No answer per day" values={series("expired")} />}
        value={model.expired}
      />
    </MetricGrid>
  );
}

function Outcomes({ model }: { model: ActivityModel }) {
  return (
    <Section
      label="What happened"
      right={<span className="text-eyebrow text-muted-foreground">By day</span>}
    >
      <Panel bodyClassName="flex flex-col gap-3">
        <DayStackedBars
          ariaLabel="Actions per day by outcome over the last 14 days"
          className="min-h-44"
          data={model.days}
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

function StartedBy({ row }: { row: ActivityRow }) {
  if (row.starter === "unknown") {
    return (
      <span className="inline-flex items-center gap-1.5 text-subtle">
        <CircleHelp aria-hidden="true" className="size-3.5" />
        Unknown
      </span>
    );
  }
  return (
    <span className="flex min-w-0 flex-col">
      <span className="truncate">{STARTER_LABEL[row.starter]}</span>
      {row.source ? <span className="truncate text-eyebrow text-subtle">{row.source}</span> : null}
    </span>
  );
}

/** The action a log row records: its icon, its name and the exact detail. */
export function ActivityActionCell({
  icon: Icon,
  action,
  detail,
}: {
  icon: LucideIcon;
  action: string;
  detail: string;
}) {
  return (
    <TableCell>
      <span className="flex min-w-0 items-center gap-2.5">
        <Icon aria-hidden="true" className="size-4 shrink-0 text-subtle" />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body text-fg">{action}</span>
          <span className="truncate font-mono text-mono-id text-subtle">{detail}</span>
        </span>
      </span>
    </TableCell>
  );
}

export function ActivityLogHeader() {
  return (
    <TableHeader>
      <TableRow>
        <TableHead>When</TableHead>
        <TableHead>What</TableHead>
        <TableHead>Agent</TableHead>
        <TableHead>Started by</TableHead>
        <TableHead className="text-right">Result</TableHead>
      </TableRow>
    </TableHeader>
  );
}

function Row({ row, agent }: { row: ActivityRow; agent: ActivityAgentMark }) {
  const Icon = ACTION_ICON[row.action] ?? SquareTerminal;
  const result = RESULT[row.result];
  return (
    <TableRow>
      <TableCell className="w-16 font-mono text-mono-id text-subtle">{row.when}</TableCell>
      <ActivityActionCell action={row.action} detail={row.detail} icon={Icon} />
      <TableCell>
        <span className="inline-flex items-center gap-2">
          {agent.avatar}
          {agent.label}
        </span>
      </TableCell>
      <TableCell>
        <StartedBy row={row} />
      </TableCell>
      <TableCell className="text-right">
        <Pill size="sm" tone={result.tone}>
          {result.label}
        </Pill>
      </TableCell>
    </TableRow>
  );
}

function Log({
  rows,
  agentMark,
}: {
  rows: readonly ActivityRow[];
  agentMark: (name: string, size: number) => ActivityAgentMark;
}) {
  const byDay = new Map<string, ActivityRow[]>();
  for (const row of rows) byDay.set(row.day, [...(byDay.get(row.day) ?? []), row]);
  return (
    <Panel bodyClassName="p-0">
      <Table>
        <ActivityLogHeader />
        <TableBody>
          {[...byDay].map(([day, members]) => (
            <Fragment key={day}>
              <TableRow>
                <TableCell
                  className="bg-surface-2/40 py-1.5 text-eyebrow text-muted-foreground"
                  colSpan={5}
                >
                  {day}
                </TableCell>
              </TableRow>
              {members.map(row => (
                <Row agent={agentMark(row.agentName, 22)} key={row.id} row={row} />
              ))}
            </Fragment>
          ))}
        </TableBody>
      </Table>
    </Panel>
  );
}

export interface ActivityPageViewProps {
  filter: ActivityFilter;
  model: ActivityModel;
  rows: readonly ActivityRow[];
  truncated: boolean;
  workspaceName: string | null;
  agentMark: (name: string, size: number) => ActivityAgentMark;
}

export function ActivityPageView({
  filter,
  model,
  rows,
  truncated,
  workspaceName,
  agentMark,
}: ActivityPageViewProps) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto" data-testid="activity-page">
      <div className={cn(PAGE_CONTENT_GUTTER, HOME_BODY_CLASS)}>
        <HomePageMeta workspaceName={workspaceName} />
        {model.total === 0 ? (
          <Panel>
            <Empty
              description="When a check-in, automation or extension does something on its own, it shows up here."
              icon={MoonStar}
              title="Nothing ran on its own yet"
            />
          </Panel>
        ) : (
          <div className={HOME_ZONE_STACK_CLASS}>
            <h2 className="text-detail-h1 font-medium text-fg-strong">{model.sentence}</h2>
            <Stats filter={filter} model={model} />
            <HomePulseHeatmap pulse={model.pulse} />
            <div className={HOME_ZONE_PAIR_CLASS}>
              <Outcomes model={model} />
              <div className="flex flex-col gap-5">
                <Section label="Who started it">
                  <Panel>
                    <StatusBreakdown
                      items={model.starters.map(item => ({
                        id: item.key,
                        label: item.label,
                        value: item.value,
                      }))}
                    />
                  </Panel>
                </Section>
                <Section label="Which agents">
                  <Panel>
                    <StatusBreakdown
                      items={model.agents.map(({ name, value }) => {
                        const mark = agentMark(name, 18);
                        return { id: name, label: mark.label, swatch: mark.avatar, value };
                      })}
                    />
                  </Panel>
                </Section>
              </div>
            </div>
            <Section
              count={rows.length}
              label="Everything it did"
              right={
                truncated ? (
                  <span className="text-eyebrow text-muted-foreground">Latest 1,000</span>
                ) : undefined
              }
            >
              {rows.length > 0 ? (
                <Log agentMark={agentMark} rows={rows} />
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
  );
}
