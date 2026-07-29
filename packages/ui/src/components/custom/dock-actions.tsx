"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

function DockActions({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dock-actions"
      className={cn("mt-2.5 flex items-center gap-[7px]", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export { DockActions };
