"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface DockStatusProps extends React.ComponentProps<"div"> {
  /** `danger` for retryable errors; default is the quiet submitting line. */
  tone?: "neutral" | "danger";
}

/** Quiet status line under the actions — submitting / retryable error. */
function DockStatus({ tone = "neutral", children, className, ...props }: DockStatusProps) {
  return (
    <div
      data-slot="dock-status"
      data-tone={tone}
      className={cn(
        "mt-[7px] text-[11px]",
        tone === "danger" ? "text-danger" : "text-subtle",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export { DockStatus };
