import * as React from "react";

import { cn } from "../../lib/utils";
import {
  DAY_CHART_AXIS_LINE,
  DAY_CHART_BAR_RADIUS,
  DAY_CHART_TICK,
  dayChartEndpointTickFormatter,
} from "./day-chart-axis";
import { CHART_TOOLTIP_CONTENT_STYLE } from "./chart-tooltip-style";

export interface DayStackedBarsSeries {
  key: string;
  /** Token-derived fill (`var(--color-…)`) — color meaning stays with the caller. */
  fill: string;
  label?: string;
}

export type DayStackedBarsDatum = { date: string } & Record<string, number | string>;

export interface DayStackedBarsProps extends Omit<React.ComponentProps<"div">, "children"> {
  data: ReadonlyArray<DayStackedBarsDatum>;
  series: ReadonlyArray<DayStackedBarsSeries>;
  height?: number;
  ariaLabel: string;
  withTooltip?: boolean;
}

const LazyBars = React.lazy(async () => {
  const { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis } = await import("recharts");

  function Bars({
    data,
    series,
    withTooltip,
  }: Pick<DayStackedBarsProps, "data" | "series" | "withTooltip">) {
    return (
      <ResponsiveContainer height="100%" width="100%">
        <BarChart
          barCategoryGap="18%"
          data={[...data]}
          margin={{ top: 4, right: 0, bottom: 0, left: 0 }}
        >
          <XAxis
            axisLine={DAY_CHART_AXIS_LINE}
            dataKey="date"
            interval={0}
            tick={DAY_CHART_TICK}
            tickFormatter={dayChartEndpointTickFormatter(data.length)}
            tickLine={false}
          />
          {withTooltip ? (
            <Tooltip
              contentStyle={CHART_TOOLTIP_CONTENT_STYLE}
              cursor={{ fill: "var(--color-row-hover)" }}
              isAnimationActive={false}
            />
          ) : null}
          {series.map(entry => (
            <Bar
              dataKey={entry.key}
              fill={entry.fill}
              isAnimationActive={false}
              key={entry.key}
              name={entry.label ?? entry.key}
              radius={DAY_CHART_BAR_RADIUS}
              stackId="day"
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    );
  }

  return { default: Bars };
});

/**
 * Stacked per-day bars, lazily loading recharts. Fills come from the caller's
 * typed series — the chart itself never picks a palette.
 */
export function DayStackedBars({
  data,
  series,
  height = 150,
  ariaLabel,
  withTooltip = true,
  className,
  style,
  ...props
}: DayStackedBarsProps) {
  return (
    <div
      aria-label={ariaLabel}
      className={cn("w-full", className)}
      data-slot="day-stacked-bars"
      role="img"
      style={{ height, ...style }}
      {...props}
    >
      <React.Suspense fallback={<div aria-hidden="true" className="size-full" />}>
        <LazyBars data={data} series={series} withTooltip={withTooltip} />
      </React.Suspense>
    </div>
  );
}
