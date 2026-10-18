"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

function DockActions({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="dock-actions" className={cn("flex items-center gap-2", className)} {...props}>
      {children}
    </div>
  );
}

export { DockActions };
