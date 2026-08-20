"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/** Mono command/subject block on the code wash — what the decision is about. */
function DockPre({ children, className, ...props }: React.ComponentProps<"pre">) {
  return (
    <pre
      data-slot="dock-pre"
      className={cn(
        "max-h-[130px] overflow-auto rounded-md border border-line bg-chat-fill-code",
        "px-2.5 py-2 font-mono text-small-body leading-relaxed text-fg",
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
