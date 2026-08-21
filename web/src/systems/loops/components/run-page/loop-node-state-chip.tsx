import { createElement } from "react";
import type { ComponentProps } from "react";

import { cn, Pill, PillDot } from "@compozy/ui";

import type { LoopStateChip } from "../../lib/loop-run-state-copy";

interface LoopNodeStateChipProps extends Omit<
  ComponentProps<typeof Pill>,
  "tone" | "children" | "pulse"
> {
  chip: LoopStateChip;
}

/**
 * One node state, on every surface that shows one.
 *
 * Tone, glyph and the literal state word always travel together — a chip that
 * carried only colour would be unreadable to anyone who cannot separate warning
 * from danger, and unreadable in a screenshot.
 *
 * `form` is what keeps `pending` and `not_taken` apart while both stay on the
 * calm ramp: pending is an outline with nothing filled in yet, not-taken is
 * filled but dimmed, because the run has settled the question. Neither uses
 * opacity, which would drop the word below the contrast floor.
 */
const FORM_CLASS = {
  solid: "",
  hollow: "border-dashed bg-transparent text-subtle",
  absent: "bg-canvas-tint text-faint",
} as const;

export function LoopNodeStateChip({ chip, className, ...props }: LoopNodeStateChipProps) {
  return (
    <Pill
      className={cn(FORM_CLASS[chip.form], className)}
      data-state={chip.state}
      data-form={chip.form}
      data-testid={`loop-state-chip-${chip.state}`}
      tone={chip.tone}
      {...props}
    >
      {/* The live accent is a pulsing dot rather than a glyph, so motion marks
          the one running thing and nothing else on the page competes with it.
          Which state that is belongs to the state model, not to this render: a
          hardcoded `pulse` would animate every future icon-less state too. */}
      {chip.icon ? (
        createElement(chip.icon, { "aria-hidden": true, className: "size-3 shrink-0" })
      ) : (
        <PillDot pulse={chip.pulse} tone={chip.tone} />
      )}
      {chip.label}
    </Pill>
  );
}
