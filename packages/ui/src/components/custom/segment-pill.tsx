"use client";

import { m } from "motion/react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { SEGMENT_PILL_DURATION, useSegmentPill } from "./hooks/use-segment-pill";

export interface SegmentPillProps extends Omit<React.ComponentProps<"span">, "children"> {
  /**
   * Index of the selected segment among the parent's children marked with
   * `SEGMENT_PILL_TARGET`, or `null` to hide the pill. The parent must be the
   * positioned box the segments sit in.
   */
  index: number | null;
  /** Seconds for one full travel; defaults to `SEGMENT_PILL_DURATION`. */
  duration?: number;
  /** Classes for the plate itself (its fill); defaults to `bg-surface-2`. */
  className?: string;
}

/**
 * The selection plate of a segmented control that travels between segments
 * like a rubber band: the leading edge stretches toward the new segment, then
 * the trailing edge catches up. Drawn as two fixed caps and a scaled body, so
 * every frame is a transform — the radius never distorts and nothing lays out.
 * Reduced motion jumps straight to the target.
 */
function SegmentPill({
  index,
  duration = SEGMENT_PILL_DURATION,
  className,
  ...props
}: SegmentPillProps) {
  const { ref, opacity, left, cap, bodyX, bodyScale, rightCapX } = useSegmentPill(index, duration);

  const part = cn("absolute inset-y-0 left-0 bg-surface-2", className);
  return (
    <span
      ref={ref}
      aria-hidden="true"
      data-slot="segment-pill"
      data-state={index === null ? "hidden" : "shown"}
      className="pointer-events-none absolute inset-0"
      {...props}
    >
      <m.span className="absolute inset-0" style={{ opacity }}>
        <m.span className={cn(part, "rounded-l-full")} style={{ x: left, width: cap }} />
        <m.span className={cn(part, "w-px origin-left")} style={{ x: bodyX, scaleX: bodyScale }} />
        <m.span className={cn(part, "rounded-r-full")} style={{ x: rightCapX, width: cap }} />
      </m.span>
    </span>
  );
}

export { SegmentPill };
