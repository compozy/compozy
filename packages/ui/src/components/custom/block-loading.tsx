"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Spinner } from "../spinner";

type BlockLoadingSize = "sm" | "md";
type BlockLoadingSurface = "bare" | "panel";

interface BlockLoadingProps extends React.ComponentProps<"div"> {
  size?: BlockLoadingSize;
  surface?: BlockLoadingSurface;
  label?: string;
  /**
   * Render `label` as visible text under the spinner. Leave it off where the
   * layout has no room for a line of text; the spinner keeps `label` as its
   * accessible name either way.
   */
  showLabel?: boolean;
}

const SIZE_CLASSES: Record<BlockLoadingSize, string> = {
  sm: "min-h-28",
  md: "min-h-48",
};

const SURFACE_CLASSES: Record<BlockLoadingSurface, string> = {
  bare: "",
  panel: "rounded-lg border border-line bg-canvas-soft",
};

function BlockLoading({
  size = "md",
  surface = "panel",
  label = "Loading",
  showLabel = false,
  className,
  ...props
}: BlockLoadingProps) {
  return (
    <div
      data-slot="block-loading"
      data-size={size}
      data-surface={surface}
      className={cn(
        "flex min-w-0 flex-col items-center justify-center gap-2.5",
        SIZE_CLASSES[size],
        SURFACE_CLASSES[surface],
        className
      )}
      {...props}
    >
      <Spinner aria-label={label} className="size-5 text-subtle" />
      {showLabel ? (
        <span
          aria-hidden="true"
          data-slot="block-loading-label"
          className="text-small-body text-muted"
        >
          {label}
        </span>
      ) : null}
    </div>
  );
}

export { BlockLoading };
export type { BlockLoadingProps, BlockLoadingSize, BlockLoadingSurface };
