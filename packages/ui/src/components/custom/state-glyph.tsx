"use client";

import { useReducedMotionConfig } from "motion/react";
import type * as React from "react";

import { cn } from "../../lib/utils";

export type StateGlyphState = "running" | "queued" | "done" | "attention" | "idle";
/** `md` = 14 px (list status columns); `sm` = 12 px (dense transcript tool rows). */
export type StateGlyphSize = "sm" | "md";

export interface StateGlyphProps extends Omit<React.ComponentProps<"svg">, "children"> {
  state: StateGlyphState;
  size?: StateGlyphSize;
  /**
   * Accessible name. Omit it when a visible label sits beside the glyph (the
   * default, decorative case); pass it when the glyph stands alone.
   */
  label?: string;
}

const SIZE_CLASS: Record<StateGlyphSize, string> = {
  sm: "size-3",
  md: "size-3.5",
};

const STATE_CLASS: Record<StateGlyphState, string> = {
  running: "text-success",
  queued: "text-line-strong",
  done: "text-success",
  attention: "text-accent",
  idle: "text-subtle",
};

// All states share one 16-unit box so labels beside them align row to row.
const RING_RADIUS = 6.25;
const RING_CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS;

function StateGlyphMark({ state }: { state: StateGlyphState }) {
  switch (state) {
    case "running":
      return (
        <>
          <circle cx="8" cy="8" r={RING_RADIUS} opacity="0.22" />
          <circle
            cx="8"
            cy="8"
            r={RING_RADIUS}
            strokeDasharray={`${RING_CIRCUMFERENCE / 4} ${RING_CIRCUMFERENCE}`}
            strokeLinecap="round"
          />
        </>
      );
    case "queued":
      return <circle cx="8" cy="8" r={RING_RADIUS} strokeDasharray="2.2 2.2" />;
    case "done":
      return (
        <>
          <circle cx="8" cy="8" r="7" fill="currentColor" stroke="none" />
          <path
            d="M5 8.25l2 2 4-4.25"
            className="stroke-canvas"
            strokeWidth="1.75"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </>
      );
    case "attention":
    case "idle":
      return <circle cx="8" cy="8" r="4" fill="currentColor" stroke="none" />;
  }
}

/**
 * Work-state mark from the shell-rail status vocabulary: a mint spinner ring
 * (running), a dashed ring (queued), a filled mint check (done), a Compozy
 * orange dot (attention / needs you) and a subtle dot (idle). Warning amber is
 * never a state glyph — it stays reserved for real warnings. Under reduced
 * motion the running ring holds still as a quarter arc, which still reads as
 * in progress.
 */
function StateGlyph({ state, size = "md", label, className, ...props }: StateGlyphProps) {
  const reduced = useReducedMotionConfig();
  const spinning = state === "running" && !reduced;
  return (
    <svg
      data-slot="state-glyph"
      data-state={state}
      data-size={size}
      data-spinning={spinning ? "true" : undefined}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : "true"}
      className={cn(
        "inline-block shrink-0",
        SIZE_CLASS[size],
        STATE_CLASS[state],
        spinning && "animate-spin",
        className
      )}
      {...props}
    >
      <StateGlyphMark state={state} />
    </svg>
  );
}

export { StateGlyph };
