"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

const PRIORITY_LEVELS = ["low", "medium", "high", "urgent"] as const;
export type PriorityLevel = (typeof PRIORITY_LEVELS)[number];

export interface PriorityBarsProps extends React.ComponentProps<"span"> {
  level: PriorityLevel;
  ariaLabel?: string;
}

/**
 * Color-from-level mapping The glyph always renders three
 * ascending bars (4 / 8 / 12 px); the `level` prop drives the bar fill color,
 * not the fill count. `medium` and `normal` (alias retained externally) read as
 * the resting `--fg`; `high` and `urgent` escalate via the warning / danger
 * signal tokens; `low` recedes into `--faint`.
 */
const LEVEL_FILL: Record<PriorityLevel, string> = {
  low: "bg-faint",
  medium: "bg-fg",
  high: "bg-warning",
  urgent: "bg-danger",
};

const BAR_HEIGHTS = ["h-1", "h-2", "h-3"] as const;

function PriorityBars({ level, ariaLabel, className, ...props }: PriorityBarsProps) {
  const fillClass = LEVEL_FILL[level];
  return (
    <span
      data-slot="priority-bars"
      role="img"
      aria-label={ariaLabel ?? `${level} priority`}
      data-level={level}
      className={cn("inline-flex items-end gap-px", className)}
      {...props}
    >
      {BAR_HEIGHTS.map(heightClass => (
        <span
          key={heightClass}
          data-slot="priority-bars-bar"
          aria-hidden="true"
          className={cn("w-0.5 rounded-xs", heightClass, fillClass)}
        />
      ))}
    </span>
  );
}

export { PriorityBars };
