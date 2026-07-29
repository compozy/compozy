"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

function DockTitle({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dock-title"
      className={cn("text-small-body font-medium text-fg", className)}
      {...props}
    >
      {children}
    </span>
  );
}

export { DockTitle };
