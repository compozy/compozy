import type * as React from "react";

import { cn } from "../lib/utils";

export type SurfaceSize = "default" | "compact";

export interface SurfaceProps extends React.ComponentProps<"div"> {
  /** `default` = px-5 py-4 card padding; `compact` = px-4 py-3 for dense tiles. */
  size?: SurfaceSize;
}

/**
 * The canonical flat tile: `rounded-lg` on `--canvas-soft`, no border or
 * shadow. Card-like surfaces (StatusCard, RunCard, DescriptionCard, Metric)
 * compose this instead of re-copying the `rounded-lg bg-canvas-soft px-5 py-4`
 * tuple. Content layout (flex, gap) stays with the composing consumer.
 */
function Surface({ size = "default", className, ...props }: SurfaceProps) {
  return (
    <div
      data-slot="surface"
      data-size={size}
      className={cn(
        "rounded-lg bg-canvas-soft",
        size === "compact" ? "px-4 py-3" : "px-5 py-4",
        className
      )}
      {...props}
    />
  );
}

export { Surface };
