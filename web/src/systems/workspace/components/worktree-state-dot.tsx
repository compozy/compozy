import { cn } from "@compozy/ui";

import type { WorktreeDisplayState } from "../types";

/** Non-ready states only. Ready renders a quiet success dot, not this component. */
export type WorktreeDotState = Exclude<WorktreeDisplayState, "ready">;

/**
 * Single source of shape truth for nested rows — the glyph geometry mirrors
 * `WorktreeStateChip`'s leading dot so a state reads identically at both
 * densities. Colour alone never carries the state.
 */
const DOT_SHAPE: Record<WorktreeDotState, string> = {
  // Filled diamond — deliberately NOT the hollow ring, which means discovered.
  pending: "size-[5.5px] rotate-45 rounded-[1px] bg-warning",
  discovered: "size-[7px] rounded-full shadow-[inset_0_0_0_1.4px_var(--color-info)]",
  missing: "size-[7px] rounded-full shadow-[inset_0_0_0_1.4px_var(--color-info)]",
  error: "size-[7px] rounded-[1.5px] bg-danger",
  failed: "size-[7px] rounded-[1.5px] bg-danger",
};

export interface WorktreeStateDotProps extends Omit<React.ComponentProps<"span">, "children"> {
  state: WorktreeDotState;
}

export function WorktreeStateDot({ state, className, ...props }: WorktreeStateDotProps) {
  return (
    <span
      aria-hidden="true"
      data-slot="worktree-state-dot"
      data-state={state}
      className={cn("inline-block shrink-0", DOT_SHAPE[state], className)}
      {...props}
    />
  );
}
