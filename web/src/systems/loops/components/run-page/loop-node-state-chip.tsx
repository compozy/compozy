import type { ComponentProps } from "react";

import { cn, StateGlyph } from "@compozy/ui";

import type { LoopStateChip } from "../../lib/loop-run-state-copy";

interface LoopNodeStateChipProps extends Omit<ComponentProps<"span">, "children"> {
  chip: LoopStateChip;
}

/**
 * One node state, on every surface that shows one: the canonical state glyph
 * and the literal state word always travel together, so the state stays
 * readable without colour and in a screenshot.
 *
 * `form` keeps `pending` and `not_taken` apart while both stay calm: pending is
 * the dashed "nothing yet" ring, not-taken is the idle dot with a quieter word,
 * because the run has settled the question. Neither uses opacity, which would
 * drop the word below the contrast floor.
 */
export function LoopNodeStateChip({ chip, className, ...props }: LoopNodeStateChipProps) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-small-body",
        chip.form === "absent" ? "text-subtle" : "text-fg-2",
        className
      )}
      data-form={chip.form}
      data-slot="loop-state-chip"
      data-state={chip.state}
      data-testid={`loop-state-chip-${chip.state}`}
      {...props}
      aria-label={chip.label}
    >
      <StateGlyph size="sm" state={chip.glyph} />
      {chip.label}
    </span>
  );
}
