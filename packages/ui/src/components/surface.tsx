import type * as React from "react";

import { cn } from "../lib/utils";

export type SurfaceSize = "default" | "compact";
export type SurfaceVariant = "card" | "sunken";

export interface SurfaceProps extends React.ComponentProps<"div"> {
  /** `default` = px-5 py-4 card padding; `compact` = px-4 py-3 for dense tiles. */
  size?: SurfaceSize;
  /**
   * `card` (default) lifts content on `--canvas` with `shadow-card`. `sunken` is
   * the recessed inset for secondary lists (tool rows, command previews): no
   * border and no shadow on `--sunken`.
   */
  variant?: SurfaceVariant;
}

/**
 * The canonical tile at `rounded-lg`, one of the two surface levels. Card-like
 * surfaces (StatusCard, RunCard, DescriptionCard, Metric) compose this instead
 * of re-copying the surface tuple. Content layout (flex, gap) stays with the
 * composing consumer.
 */
function Surface({ size = "default", variant = "card", className, ...props }: SurfaceProps) {
  return (
    <div
      data-slot="surface"
      data-size={size}
      data-variant={variant}
      className={cn(
        "rounded-lg",
        variant === "sunken" ? "bg-sunken" : "bg-canvas shadow-card",
        size === "compact" ? "px-4 py-3" : "px-5 py-4",
        className
      )}
      {...props}
    />
  );
}

export { Surface };
