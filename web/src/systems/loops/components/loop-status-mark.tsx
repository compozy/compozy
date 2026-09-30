import type * as React from "react";

import { cn, StateGlyph } from "@compozy/ui";

import { loopStatusGlyph, loopStatusLabel } from "../lib/loop-formatters";

export interface LoopStatusMarkProps extends Omit<React.ComponentProps<"span">, "children"> {
  /** Raw `loop_run.status` from the daemon; unknown values render an idle, label-only mark. */
  status?: string | null;
  /** `sm` (default) sits in heads and rows; `xs` in dense pickers. */
  size?: "sm" | "xs";
}

/**
 * A Loop run status as the canonical state glyph beside its literal label.
 * Glyph and label come from the single mapping in `loop-formatters`, so the
 * status stays truthful everywhere it renders (never a coerced state).
 */
export function LoopStatusMark({ status, size = "sm", className, ...props }: LoopStatusMarkProps) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-fg-2",
        size === "xs" ? "text-eyebrow" : "text-small-body",
        className
      )}
      data-slot="loop-status"
      data-state={loopStatusGlyph(status)}
      {...props}
    >
      <StateGlyph size={size === "xs" ? "sm" : "md"} state={loopStatusGlyph(status)} />
      {loopStatusLabel(status)}
    </span>
  );
}
