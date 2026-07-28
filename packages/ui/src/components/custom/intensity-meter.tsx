import type * as React from "react";

import { cn } from "../../lib/utils";

/**
 * Discrete intensity meter — seven ascending bars. `position` (1..7) fills bars
 * with `accent-strong`; `hollow` renders every bar faint for a default/unset
 * state. A generic signal-strength primitive (reasoning effort, confidence,
 * load, …); accent-strong is the only fill.
 */
const BAR_HEIGHTS = ["h-px", "h-0.5", "h-1", "h-1.5", "h-2", "h-2.5", "h-3"] as const;

export interface IntensityMeterProps extends Omit<React.ComponentProps<"span">, "children"> {
  /** 1-based fill count (0 = none filled). */
  position: number;
  /** When true, every bar renders in the faint "default" tone. */
  hollow?: boolean;
}

function IntensityMeter({ position, hollow = false, className, ...props }: IntensityMeterProps) {
  const filledCount = hollow ? 0 : Math.max(0, Math.min(BAR_HEIGHTS.length, position));
  return (
    <span
      aria-hidden="true"
      data-slot="intensity-meter"
      data-position={filledCount}
      data-hollow={hollow ? "true" : "false"}
      className={cn("inline-flex h-3 items-end gap-0.5", className)}
      {...props}
    >
      {BAR_HEIGHTS.map((heightClass, index) => {
        const filled = !hollow && index + 1 <= position;
        return (
          <span
            key={heightClass}
            data-fill={hollow ? "hollow" : filled ? "on" : "off"}
            className={cn(
              "w-0.5 rounded-full",
              heightClass,
              hollow ? "bg-line-strong" : filled ? "bg-accent-strong" : "bg-faint"
            )}
          />
        );
      })}
    </span>
  );
}

export { IntensityMeter };
