"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/** Static mono deadline hint ("times out 14:32") — it never ticks. */
function DockDeadline({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dock-deadline"
      className={cn("ml-auto font-mono text-[10px] text-faint tabular-nums", className)}
      {...props}
    >
      {children}
    </span>
  );
}

export { DockDeadline };
