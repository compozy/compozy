import { CornerDownRight } from "lucide-react";

import { Skeleton, StateGlyph } from "@compozy/ui";

import { useLoopChildRun } from "../../../hooks/use-loop-child-run";
import { formatClockDuration } from "../../../lib/loop-run-usage";
import { loopStatusGlyph, loopStatusLabel } from "../../../lib/loop-formatters";

/**
 * Where a step's child run is, on the step's own graph card.
 *
 * The card is the button that opens the node panel, so this line is a reading,
 * never a control: the step the child is on and how long it has sat there, or
 * the child's status once it has nothing left to be on. The panel holds the
 * full row and anything the child started in turn.
 */
export function LoopDagChildLine({ runId }: { runId: string }) {
  const { summary, isLoading } = useLoopChildRun(runId);
  if (!summary) {
    return isLoading ? <Skeleton className="h-3 w-24" /> : null;
  }
  const step = summary.currentStep;
  const onStep = step?.onStepSeconds == null ? null : formatClockDuration(step.onStepSeconds);
  const spoken = step
    ? `Child run ${summary.loopName} is at ${step.name}${step.detail}${onStep ? `, ${onStep} on this step` : ""}`
    : `Child run ${summary.loopName}: ${loopStatusLabel(summary.status)}`;
  return (
    <span
      className="flex min-w-0 items-center gap-1.5 text-form-hint text-muted"
      data-testid="loop-dag-child-line"
      title={spoken}
    >
      <span className="sr-only">{spoken}</span>
      {/* The line is the child's, not the step's: it hangs from the card. */}
      <CornerDownRight aria-hidden="true" className="size-3 shrink-0 text-faint" />
      <StateGlyph
        className="shrink-0"
        size="sm"
        state={step ? step.chip.glyph : loopStatusGlyph(summary.status)}
      />
      <span aria-hidden="true" className="min-w-0 truncate">
        {step ? step.name : `child ${loopStatusLabel(summary.status).toLowerCase()}`}
      </span>
      {onStep ? (
        <span
          aria-hidden="true"
          className="ml-auto shrink-0 font-mono text-mono-id tabular-nums text-subtle"
        >
          {onStep}
        </span>
      ) : null}
    </span>
  );
}
