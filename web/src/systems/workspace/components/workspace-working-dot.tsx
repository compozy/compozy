import { useReducedMotionConfig } from "motion/react";

import { cn, StatusDot } from "@compozy/ui";

import { workspaceWorkingLabel } from "../lib/workspace-working-label";

export interface WorkspaceWorkingDotProps {
  /** Sessions working in the workspace; `0` renders nothing. */
  count: number;
  /** `corner` cuts the dot into a mark's top corner; `inline` sits in a label's flow. */
  placement?: "corner" | "inline";
  className?: string;
}

/**
 * Something is working in the workspace right now. Mint is the running
 * vocabulary; in the corner of a mark the ring cuts it out of the well.
 */
export function WorkspaceWorkingDot({
  count,
  placement = "corner",
  className,
}: WorkspaceWorkingDotProps) {
  const reduced = useReducedMotionConfig();
  if (count <= 0) return null;

  return (
    <StatusDot
      tone="success"
      size="sm"
      label={workspaceWorkingLabel(count)}
      data-slot="workspace-working-dot"
      className={cn(
        "pointer-events-none",
        placement === "corner" && "absolute -top-0.5 -right-0.5 z-10 ring-2 ring-popover",
        !reduced && "animate-pulse",
        className
      )}
    />
  );
}
