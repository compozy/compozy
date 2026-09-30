"use client";

import type * as React from "react";

import { cn } from "../../lib/utils";
import {
  ToolCallRowGroupContext,
  type ToolCallRowGroupContextValue,
} from "./hooks/use-tool-call-row-group";

export interface ToolCallRowGroupProps extends React.ComponentProps<"div"> {
  /** Stop the running glyphs inside the group without reduced motion (US-018.EC-2). */
  still?: boolean;
}

/**
 * The sunken tool panel from the shell-rail transcript: a recessed `bg-sunken`
 * inset with no border that switches every `ToolCallRow` inside it to inset
 * density — 34 px rows, 16 px sides, an `fg` verb, a 13 px mono preview.
 */
function ToolCallRowGroup({ still = false, className, children, ...props }: ToolCallRowGroupProps) {
  const value: ToolCallRowGroupContextValue = { density: "inset", still };
  return (
    <div
      data-slot="tool-call-row-group"
      data-still={still ? "true" : undefined}
      className={cn("flex min-w-0 flex-col overflow-hidden rounded-lg bg-sunken py-1.5", className)}
      {...props}
    >
      <ToolCallRowGroupContext value={value}>{children}</ToolCallRowGroupContext>
    </div>
  );
}

export { ToolCallRowGroup };
