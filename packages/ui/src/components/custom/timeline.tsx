"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface TimelineProps extends React.ComponentProps<"ol"> {
  ariaLabel?: string;
}

function Timeline({ ariaLabel, className, children, ...props }: TimelineProps) {
  return (
    <ol
      data-slot="timeline"
      aria-label={ariaLabel}
      className={cn(
        "relative flex flex-col before:absolute before:top-2 before:bottom-2 before:left-2 before:w-px before:bg-line",
        className
      )}
      {...props}
    >
      {children}
    </ol>
  );
}

export { Timeline };
