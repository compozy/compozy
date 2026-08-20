"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

function DockMeta({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dock-meta"
      className={cn(
        "mt-[5px] text-form-label text-subtle",
        "[&_code]:font-mono [&_code]:text-badge [&_code]:text-muted",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export { DockMeta };
