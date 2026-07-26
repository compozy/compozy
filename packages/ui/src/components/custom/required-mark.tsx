"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Eyebrow } from "./eyebrow";

export interface RequiredMarkProps extends React.ComponentProps<"span"> {
  /** Overrides the affix text; defaults to the `required` contract label. */
  children?: React.ReactNode;
}

/**
 * Accent affix marking a field the contract rejects when empty.
 *
 * The designed entity editors mark required fields with an accent eyebrow next
 * to the label rather than an asterisk, so the requirement reads as words
 * instead of relying on a glyph. Pair it with real submit gating — the affix
 * announces the rule, it does not enforce it.
 */
function RequiredMark({ children = "required", className, ...props }: RequiredMarkProps) {
  return (
    <Eyebrow
      data-slot="required-mark"
      className={cn("ml-1.5 text-accent-strong", className)}
      {...props}
    >
      {children}
    </Eyebrow>
  );
}

export { RequiredMark };
