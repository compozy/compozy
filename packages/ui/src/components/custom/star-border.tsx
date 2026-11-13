"use client";

import { useReducedMotionConfig } from "motion/react";
import type * as React from "react";

import { cn } from "../../lib/utils";

export interface StarBorderProps extends React.ComponentProps<"span"> {
  /**
   * Runs the two glows travelling the rim. `false` keeps the static brand rim
   * for a control that is present but busy; reduced motion always does.
   */
  animated?: boolean;
}

/**
 * Rims one control in the brand gradient (`--gradient-brand`) and, while
 * `animated`, sends two soft glows along its top and bottom edges — the call to
 * action that must not be missed. The effect is a decorative overlay on a 1px
 * ring: the wrapped control keeps its own fill, size, hit area and focus ring,
 * and owns every interaction. Reduced motion (the system preference, the
 * host's `data-reduce-motion`, or `UIProvider`) shows the static rim only.
 * Styles live in `tokens-runtime.css` (`star-border`).
 */
function StarBorder({ animated = true, className, children, ...props }: StarBorderProps) {
  const reduced = useReducedMotionConfig();
  const moving = animated && !reduced;
  return (
    <span
      data-slot="star-border"
      data-animated={moving ? "true" : "false"}
      className={cn("star-border rounded-pill", className)}
      {...props}
    >
      {children}
      <span aria-hidden="true" data-slot="star-border-rim">
        {moving ? (
          <>
            <span data-edge="top" data-slot="star-border-glow" />
            <span data-edge="bottom" data-slot="star-border-glow" />
          </>
        ) : null}
      </span>
    </span>
  );
}

export { StarBorder };
