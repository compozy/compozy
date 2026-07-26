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
  className,
  ...props
}: BlockLoadingProps) {
  return (
    <div
      data-slot="block-loading"
      data-size={size}
      data-surface={surface}
      className={cn(
        "flex min-w-0 items-center justify-center",
        SIZE_CLASSES[size],
        SURFACE_CLASSES[surface],
        className
      )}
      {...props}
    >
      <Spinner aria-label={label} className="size-5 text-subtle" />
    </div>
  );
}

export { BlockLoading };
export type { BlockLoadingProps, BlockLoadingSize, BlockLoadingSurface };
