"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface SparklineProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Bucketed values; each value renders as a bar. Layout is deterministic. */
  values: ReadonlyArray<number>;
  /** Optional max for normalization; defaults to the largest value (or 1). */
  max?: number;
  /** Total width hint in CSS pixels; bars fill the container by default. */
  height?: number;
  /** Accessible label for screen readers. */
  ariaLabel?: string;
}

function Sparkline({
  values,
  max,
  height = 28,
  ariaLabel,
  className,
  style,
  ...props
}: SparklineProps) {
  const peak = Math.max(max ?? Math.max(0, ...values), 1);
  const safeValues = values.length === 0 ? [0] : values;
  const valueOccurrences = new Map<number, number>();
  return (
    <div
      data-slot="sparkline"
      role="img"
      aria-label={ariaLabel}
      className={cn("flex w-full items-end gap-px", className)}
      style={{ height, ...style }}
      {...props}
    >
      {safeValues.map((value, index) => {
        const ratio = Math.max(0, Math.min(1, value / peak));
        const barHeight = `${Math.round(ratio * 100)}%`;
        const occurrence = valueOccurrences.get(value) ?? 0;
        valueOccurrences.set(value, occurrence + 1);
        return (
          <span
            key={`${value}-${occurrence}`}
            data-slot="sparkline-bar"
            data-index={index}
            aria-hidden="true"
            className="flex-1 rounded-xs bg-viz-bar"
            style={{ height: barHeight }}
          />
        );
      })}
    </div>
  );
}

export { Sparkline };
