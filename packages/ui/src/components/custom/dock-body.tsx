"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

function DockBody({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="dock-body" className={cn("mt-2", className)} {...props}>
      {children}
    </div>
  );
}

export { DockBody };
