import { useReducedMotionConfig } from "motion/react";

import { cn, StatusDot } from "@compozy/ui";

import { workspaceWorkingLabel } from "../lib/workspace-working-label";

export interface WorkspaceWorkingDotProps {
  /** Sessions working in the workspace; `0` renders nothing. */
  count: number;
  className?: string;
}

/**
 * Corner mark on a workspace monogram: something is working there right now.
 * Mint is the running vocabulary; the ring cuts it out of the monogram well.
 */
export function WorkspaceWorkingDot({ count, className }: WorkspaceWorkingDotProps) {
  const reduced = useReducedMotionConfig();
  if (count <= 0) return null;

  return (
    <StatusDot
      tone="success"
      size="sm"
      label={workspaceWorkingLabel(count)}
      data-slot="workspace-working-dot"
      className={cn(
        "pointer-events-none absolute -top-0.5 -right-0.5 z-10 ring-2 ring-popover",
        !reduced && "animate-pulse",
        className
      )}
    />
  );
}
