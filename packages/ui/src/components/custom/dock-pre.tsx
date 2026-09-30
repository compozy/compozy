"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/** Mono command/subject block, recessed on `sunken` — what the decision is about. */
function DockPre({ children, className, ...props }: React.ComponentProps<"pre">) {
  return (
    <pre
      data-slot="dock-pre"
      className={cn(
        "max-h-[130px] overflow-auto rounded-lg bg-sunken",
        "px-3.5 py-3 font-mono text-meta leading-relaxed text-fg-3",
        "break-words whitespace-pre-wrap",
        className
      )}
      {...props}
    >
      {children}
    </pre>
  );
}

export { DockPre };
