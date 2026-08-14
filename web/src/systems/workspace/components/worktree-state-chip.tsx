import { cn } from "@compozy/ui";

import type { WorktreeDisplayState } from "../types";

/**
 * Ready is calm: it never carries a chip, in full rows or nests alike. The chip
 * exists to mark a state that changes what the user can do.
 */
export type WorktreeChipState = Exclude<WorktreeDisplayState, "ready">;

const CHIP_TONE: Record<WorktreeChipState, string> = {
  pending: "bg-warning-tint text-warning",
  discovered: "bg-info-tint text-info",
  // Missing is an outline, not a fill — the record survives but the tree is gone.
  missing: "bg-transparent text-info shadow-[inset_0_0_0_1px_var(--color-line-strong)]",
  error: "bg-danger-tint text-danger",
  failed: "bg-danger-tint text-danger",
};

/** Shape is redundant with colour so the state survives a colour-blind read. */
const CHIP_GLYPH: Record<WorktreeChipState, string> = {
  pending: "size-[5.5px] rotate-45 rounded-[1px] bg-current",
  discovered: "size-1.5 rounded-full shadow-[inset_0_0_0_1.4px_currentColor]",
  missing: "size-1.5 rounded-full shadow-[inset_0_0_0_1.4px_currentColor]",
  error: "size-1.5 rounded-[1.5px] bg-current",
  failed: "size-1.5 rounded-[1.5px] bg-current",
};

/**
 * `error` is a status read failure — it blocks dependent actions and does not
 * mean "dirty". `failed` is the distinct lifecycle state of a creation that
 * rolled back (TechSpec N-004); the two are never merged.
 */
const CHIP_LABEL: Record<WorktreeChipState, string> = {
  pending: "pending",
  discovered: "discovered",
  missing: "missing",
  error: "error",
  failed: "failed",
};

export interface WorktreeStateChipProps extends Omit<React.ComponentProps<"span">, "children"> {
  state: WorktreeDisplayState;
  /** `sm` is the quiet metadata form nested rows carry on their second line. */
  size?: "default" | "sm";
}

export function WorktreeStateChip({
  state,
  size = "default",
  className,
  ...props
}: WorktreeStateChipProps) {
  if (state === "ready") return null;

  return (
    <span
      data-slot="worktree-state-chip"
      data-state={state}
      data-size={size}
      className={cn(
        "inline-flex shrink-0 items-center rounded-xs leading-none",
        size === "sm"
          ? "h-4 gap-1 px-1.5 text-micro font-medium"
          : "h-[19px] gap-[5px] px-2 text-badge font-semibold",
        CHIP_TONE[state],
        className
      )}
      {...props}
    >
      <span aria-hidden="true" className={cn("inline-block shrink-0", CHIP_GLYPH[state])} />
      {CHIP_LABEL[state]}
    </span>
  );
}
