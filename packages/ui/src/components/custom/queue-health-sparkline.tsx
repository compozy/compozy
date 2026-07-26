"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface QueueHealthSparklineBucket {
  /** Bucket label rendered in tooltips (e.g. `"23h"`, `"12h"`). */
  label: string;
  /** Bar height value. */
  value: number;
  /** When true, the bar paints with `--accent-tint-strong` instead of `--bar-fill`. */
  stuck?: boolean;
}

export interface QueueHealthSparklineProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Bucketed queue depth data */
  data: ReadonlyArray<QueueHealthSparklineBucket>;
  /** Height in CSS pixels. Defaults to 96 to match the proposal baseline. */
  height?: number;
  /** Accessible label rendered as `aria-label` on the chart container. */
  ariaLabel?: string;
  /** Disable the tooltip overlay (defaults to enabled). */
  withTooltip?: boolean;
}

const DEFAULT_HEIGHT = 96;
const BAR_FILL = "var(--color-bar-fill)";
const STUCK_FILL = "var(--color-accent-tint-strong)";

const TOOLTIP_CONTENT_STYLE: React.CSSProperties = {
  background: "var(--color-canvas-soft)",
  border: "1px solid var(--color-line)",
  borderRadius: "var(--radius-sm)",
  color: "var(--color-fg)",
  fontFamily: "var(--font-mono)",
  fontSize: "var(--text-eyebrow)",
  padding: "4px 6px",
};

interface QueueHealthSparklineChartProps {
  data: ReadonlyArray<QueueHealthSparklineBucket>;
  withTooltip: boolean;
}

const QueueHealthSparklineChart = React.lazy(async () => {
  const { Bar, BarChart, Cell, ResponsiveContainer, Tooltip } = await import("recharts");
  return {
    default: function QueueHealthSparklineChartContent({
      data,
      withTooltip,
    }: QueueHealthSparklineChartProps) {
      return (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={[...data]} margin={{ top: 4, right: 0, left: 0, bottom: 0 }}>
            {withTooltip ? (
              <Tooltip
                cursor={false}
                contentStyle={TOOLTIP_CONTENT_STYLE}
                labelStyle={{ color: "var(--color-muted)" }}
                itemStyle={{ color: "var(--color-fg-strong)" }}
              />
            ) : null}
            <Bar dataKey="value" isAnimationActive={false} radius={[1, 1, 0, 0]} minPointSize={2}>
              {data.map(bucket => (
                <Cell
                  key={bucket.label}
                  data-slot="queue-health-sparkline-cell"
                  data-stuck={bucket.stuck ? "true" : undefined}
                  fill={bucket.stuck ? STUCK_FILL : BAR_FILL}
                />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      );
    },
  };
});

function QueueHealthSparkline({
  data,
  height = DEFAULT_HEIGHT,
  ariaLabel,
  withTooltip = true,
  className,
  ...props
}: QueueHealthSparklineProps) {
  return (
    <div
      data-slot="queue-health-sparkline"
      role="img"
      aria-label={ariaLabel}
      className={cn("w-full", className)}
      style={{ height }}
      {...props}
    >
      <React.Suspense fallback={<div aria-hidden="true" className="size-full" />}>
        <QueueHealthSparklineChart data={data} withTooltip={withTooltip} />
      </React.Suspense>
    </div>
  );
}

export { QueueHealthSparkline };
