"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/** Faint counter ("1/2") for stacked pending decisions. */
function DockCount({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dock-count"
      className={cn("text-badge text-faint tabular-nums", className)}
      {...props}
    >
      {children}
    </span>
  );
}

export { DockCount };
