import { Fragment } from "react";

import { Metric, MetricGrid, Panel, Section, Skeleton, SkeletonText } from "@compozy/ui";

import type { HomeUsageWindow } from "../types";
import { HomeAgentsHeader, HomePulseAxis } from "./home-zone-parts";
import {
  HOME_ACTIVITY_ROW_CLASS,
  HOME_AGENTS_NAME_CLASS,
  HOME_AGENTS_ROW_CLASS,
  HOME_KPI_LABELS,
  HOME_KPI_SPARKLINE_HEIGHT,
  HOME_OUTCOMES_HEADLINE_CLASS,
  HOME_OUTCOMES_LEGEND_CLASS,
  HOME_PULSE_GRID_CLASS,
  HOME_PULSE_GRID_COLUMNS,
  HOME_PULSE_INSIGHTS_CLASS,
  HOME_PULSE_WEEKDAY_LABELS,
  HOME_PULSE_WEEKDAY_ROWS,
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
  HOME_SYSTEM_ROW_CLASS,
  HOME_USAGE_HEADLINE_CLASS,
  homeUsageLabel,
} from "./home-zone-shape";

// The Home zones in their loading shape. Each placeholder is the live zone's twin: the same
// heading, panel and row frames (from `home-zone-shape.ts`), with the figures left as blocks.
// None carries the live zone's `data-slot`, so nothing that waits for a loaded zone can mistake
// a placeholder for it, and none renders a status region of its own: the caller wraps the area
// in one `SkeletonRegion`. Static and query-free, so the Home window can show them before its
// code has loaded.

/**
 * The four KPI tiles with their real labels and the figures left as text blocks. The usage tile
 * keeps room for its sparkline.
 */
export function HomeKpiStripSkeleton({ usageWindow }: { usageWindow: HomeUsageWindow }) {
  return (
    <MetricGrid aria-hidden="true" columns={4}>
      {[HOME_KPI_LABELS.workingNow, HOME_KPI_LABELS.needsYou, HOME_KPI_LABELS.completedToday].map(
        label => (
          <div className="min-w-0 rounded-lg" key={label}>
            <Metric
              className="animate-skeleton-reveal"
              label={label}
              labelCase="eyebrow"
              subtext={<SkeletonText className="w-28" />}
              value={<SkeletonText className="w-12" />}
            />
          </div>
        )
      )}
      <div className="min-w-0 rounded-lg">
        <Metric
          className="animate-skeleton-reveal"
          label={homeUsageLabel(usageWindow)}
          labelCase="eyebrow"
          subtext={<SkeletonText className="w-32" />}
          trailing={
            <Skeleton className="w-16 rounded-xs" style={{ height: HOME_KPI_SPARKLINE_HEIGHT }} />
          }
          value={<SkeletonText className="w-16" />}
        />
      </div>
    </MetricGrid>
  );
}

/** One placeholder run card: the live row's grid and lines. */
function HomeRunCardSkeleton() {
  return (
    <div className={HOME_RUN_ROW_CLASS}>
      <div className="min-w-0">
        <div className={HOME_RUN_TITLE_CLASS}>
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <Skeleton className="size-avatar-sm shrink-0 rounded-full" />
          <SkeletonText className="w-40" />
        </div>
        <p className={HOME_RUN_SUBTITLE_CLASS}>
          <SkeletonText className="w-56" />
        </p>
      </div>
      <SkeletonText className="w-12 text-meta" />
    </div>
  );
}

/** Cards a loading zone stands in with: what a busy desk shows without claiming a count. */
const LOADING_RUN_CARDS = ["first", "second"] as const;

/**
 * The live-work zone: its heading, the card panel and rows of blocks. The head's link is a block,
 * so nothing interactive sits in the loading region, and it carries no count it cannot know.
 */
export function HomeWorkingNowSkeleton() {
  return (
    <Section
      aria-hidden="true"
      bodyClassName="flex flex-1 flex-col gap-2.5"
      className="flex min-h-full flex-col"
      label="Working now"
      right={<Skeleton className="h-button-sm w-28" />}
    >
      <Panel bodyClassName="p-0" className="animate-skeleton-reveal overflow-hidden">
        <div className="divide-y divide-line-soft">
          {LOADING_RUN_CARDS.map(card => (
            <HomeRunCardSkeleton key={card} />
          ))}
        </div>
      </Panel>
    </Section>
  );
}

/**
 * The pulse heatmap: the real heading, weekday labels and hour axis, with one bar per weekday
 * where the 24 cells go (seven shimmering blocks rather than 168) and the insights line a busy
 * window carries.
 */
export function HomePulseHeatmapSkeleton() {
  return (
    <Section
      aria-hidden="true"
      label="When agents were busy"
      right={<span className="text-eyebrow text-muted-foreground">Last 14 days</span>}
    >
      <Panel bodyClassName="p-0" className="animate-skeleton-reveal">
        <div className="overflow-x-auto px-4 pt-4 pb-1.5">
          <div
            className={HOME_PULSE_GRID_CLASS}
            style={{ gridTemplateColumns: HOME_PULSE_GRID_COLUMNS }}
          >
            {HOME_PULSE_WEEKDAY_ROWS.map(weekday => (
              <Fragment key={weekday}>
                <span className="self-center font-mono text-micro text-faint">
                  {HOME_PULSE_WEEKDAY_LABELS[weekday]}
                </span>
                <Skeleton className="col-span-24 h-[var(--size-home-pulse-cell)] rounded-xs" />
              </Fragment>
            ))}
            <HomePulseAxis />
          </div>
        </div>
        <div className={HOME_PULSE_INSIGHTS_CLASS}>
          <SkeletonText className="w-64" />
        </div>
      </Panel>
    </Section>
  );
}

/** The run outcomes chart: the real heading and panel, with headline, chart and legend as blocks. */
export function HomeOutcomesChartSkeleton() {
  return (
    <Section
      aria-hidden="true"
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      label="How work ended"
      right={<span className="text-eyebrow text-muted-foreground">Last 14 days</span>}
    >
      <Panel bodyClassName="flex flex-1 flex-col gap-3" className="animate-skeleton-reveal flex-1">
        <div className={HOME_OUTCOMES_HEADLINE_CLASS}>
          <SkeletonText className="w-12 text-kpi-value" />
          <SkeletonText className="w-52 text-small-body" />
        </div>
        <Skeleton className="min-h-[120px] flex-1" />
        <div className={HOME_OUTCOMES_LEGEND_CLASS}>
          <SkeletonText className="w-24" />
          <SkeletonText className="w-24" />
          <SkeletonText className="w-24" />
        </div>
        <SkeletonText className="w-56 text-micro leading-relaxed" />
      </Panel>
    </Section>
  );
}

/**
 * The usage chart: the real heading and panel, with headline, chart and per-agent share as
 * blocks and a block where the window switch goes.
 */
export function HomeUsageChartSkeleton() {
  return (
    <Section
      aria-hidden="true"
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      label="Usage & cost"
      right={<Skeleton className="h-5 w-28 rounded-pill" />}
    >
      <Panel bodyClassName="flex flex-1 flex-col gap-3" className="animate-skeleton-reveal flex-1">
        <div className={HOME_USAGE_HEADLINE_CLASS}>
          <SkeletonText className="w-16 text-kpi-value" />
          <SkeletonText className="w-48 text-small-body" />
        </div>
        <Skeleton className="min-h-[100px] flex-1" />
        <div className="-mx-5 mt-auto border-t border-line-soft px-5 pt-3.5">
          <div className="mb-2.5 flex items-center justify-between gap-3">
            <SkeletonText className="w-24 text-eyebrow" />
            <SkeletonText className="w-20 text-eyebrow" />
          </div>
          <Skeleton className="h-2.5 rounded-xs" />
          <div className="mt-2.5 flex flex-wrap items-center gap-4 text-small-body">
            <SkeletonText className="w-28" />
            <SkeletonText className="w-28" />
            <SkeletonText className="w-24" />
          </div>
        </div>
      </Panel>
    </Section>
  );
}

/** Rows a loading agents zone stands in with: a small fleet, below the six the live zone caps at. */
const LOADING_AGENTS = ["first", "second", "third", "fourth"] as const;

function HomeAgentRowSkeleton() {
  return (
    <div className={HOME_AGENTS_ROW_CLASS}>
      <span className={HOME_AGENTS_NAME_CLASS}>
        <Skeleton className="size-avatar-sm shrink-0 rounded-full" />
        <SkeletonText className="w-24" />
      </span>
      <SkeletonText className="ml-auto w-4 text-small-body" />
      <SkeletonText className="ml-auto w-5 text-small-body max-[760px]:hidden" />
      <SkeletonText className="ml-auto w-4 text-small-body max-[760px]:hidden" />
      <span className="flex min-w-0 items-center gap-2.5">
        <Skeleton className="h-1 min-w-0 flex-1 rounded-pill" />
        <SkeletonText className="w-8 shrink-0 text-eyebrow" />
      </span>
    </div>
  );
}

/** The agents zone: the real heading, column heads and panel, with rows of blocks and a footer block. */
export function HomeAgentsPanelSkeleton() {
  return (
    <Section
      aria-hidden="true"
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      label="Agents"
      right={<span className="text-eyebrow text-muted-foreground">runtime · all time</span>}
    >
      <Panel
        bodyClassName="flex flex-1 flex-col p-0"
        className="animate-skeleton-reveal flex-1"
        foot={<Skeleton className="h-button-sm w-28" />}
      >
        <div className="flex flex-1 flex-col">
          <HomeAgentsHeader />
          <div className="flex flex-1 flex-col divide-y divide-line-soft">
            {LOADING_AGENTS.map(agent => (
              <HomeAgentRowSkeleton key={agent} />
            ))}
          </div>
        </div>
      </Panel>
    </Section>
  );
}

/** Rows a loading feed stands in with; the live feed shows up to eight loud events. */
const LOADING_EVENTS = [
  { id: "a", width: "w-3/4" },
  { id: "b", width: "w-2/3" },
  { id: "c", width: "w-4/5" },
  { id: "d", width: "w-3/5" },
  { id: "e", width: "w-2/3" },
] as const;

/** The activity feed: the real heading and panel, with rows of blocks in the live row's grid. */
export function HomeActivityFeedSkeleton() {
  return (
    <Section
      aria-hidden="true"
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      label="Activity"
    >
      <Panel
        bodyClassName="flex flex-1 flex-col p-0"
        className="animate-skeleton-reveal flex-1 overflow-hidden"
      >
        <div className="flex flex-1 flex-col divide-y divide-line-soft">
          {LOADING_EVENTS.map(event => (
            <div className={HOME_ACTIVITY_ROW_CLASS} key={event.id}>
              <span className="mt-1.5 flex justify-center">
                <Skeleton className="size-1.5 rounded-full" />
              </span>
              <SkeletonText className={`text-small-body ${event.width}`} />
              <SkeletonText className="w-10 text-eyebrow" />
            </div>
          ))}
        </div>
      </Panel>
    </Section>
  );
}

/** The folded system row: the dot, the verdict and the summary as blocks, in the trigger's frame. */
export function HomeSystemPanelSkeleton() {
  return (
    <Panel aria-hidden="true" bodyClassName="p-0" className="animate-skeleton-reveal">
      <div className={HOME_SYSTEM_ROW_CLASS}>
        <Skeleton className="size-2 shrink-0 rounded-full" />
        <SkeletonText className="w-36 shrink-0 text-small-body" />
        <SkeletonText className="min-w-0 flex-1 text-small-body" />
        <Skeleton className="size-3.5 shrink-0 rounded-xs" />
      </div>
    </Panel>
  );
}
