"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

function DockHead({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dock-head"
      className={cn("flex flex-wrap items-center gap-2", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export { DockHead };
