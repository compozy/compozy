import type * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Bare mono block for expanded tool detail — the `.trow__detail` grammar: 11px
 * mono at `--muted`, colored **text** spans only (`text-success` adds,
 * `text-danger` dels/errors, `text-subtle` context). No card, no header, no
 * syntax theatre.
 */
export function DetailPre({ children, className, ...props }: React.ComponentProps<"pre">) {
  return (
    <pre
      data-slot="tool-detail-pre"
      className={cn(
        "max-h-60 overflow-auto font-mono text-transcript-caption leading-prose text-muted",
        "break-words whitespace-pre-wrap",
        className
      )}
      {...props}
    >
      {children}
    </pre>
  );
}
