import * as React from "react";

import { cn } from "../../lib/utils";
import {
  DAY_CHART_AXIS_LINE,
  DAY_CHART_TICK,
  dayChartEndpointTickFormatter,
} from "./day-chart-axis";
import { CHART_TOOLTIP_CONTENT_STYLE } from "./chart-tooltip-style";

export interface DayAreaChartDatum {
  date: string;
  value: number;
}

export interface DayAreaChartProps extends Omit<React.ComponentProps<"div">, "children"> {
  data: ReadonlyArray<DayAreaChartDatum>;
  height?: number;
  ariaLabel: string;
  /** Formats the tooltip line for one day's value. */
  formatValue?: (value: number) => string;
  withTooltip?: boolean;
}

const LazyArea = React.lazy(async () => {
  const { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis } =
    await import("recharts");

  function AreaBody({
    data,
    formatValue,
    withTooltip,
  }: Pick<DayAreaChartProps, "data" | "formatValue" | "withTooltip">) {
    return (
      <ResponsiveContainer height="100%" width="100%">
        <AreaChart data={[...data]} margin={{ top: 6, right: 20, bottom: 0, left: 20 }}>
          <CartesianGrid stroke="var(--color-viz-grid)" strokeWidth={1} vertical={false} />
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
              cursor={{ stroke: "var(--color-line-focus)", strokeWidth: 1 }}
              formatter={value => {
                const numeric = typeof value === "number" ? value : Number(value);
                return [formatValue ? formatValue(numeric) : String(numeric), ""];
              }}
              isAnimationActive={false}
            />
          ) : null}
          <Area
            dataKey="value"
            fill="var(--color-viz-fill)"
            isAnimationActive={false}
            stroke="var(--color-viz-line)"
            strokeWidth={1.5}
            type="monotone"
          />
        </AreaChart>
      </ResponsiveContainer>
    );
  }

  return { default: AreaBody };
});

/**
 * Single-series per-day area chart in neutral data ink, lazily loading
 * recharts. Magnitude never borrows signal colors.
 */
export function DayAreaChart({
  data,
  height = 120,
  ariaLabel,
  formatValue,
  withTooltip = true,
  className,
  style,
  ...props
}: DayAreaChartProps) {
  return (
    <div
      aria-label={ariaLabel}
      className={cn("w-full", className)}
      data-slot="day-area-chart"
      role="img"
      style={{ height, ...style }}
      {...props}
    >
      <React.Suspense fallback={<div aria-hidden="true" className="size-full" />}>
        <LazyArea data={data} formatValue={formatValue} withTooltip={withTooltip} />
      </React.Suspense>
    </div>
  );
}
