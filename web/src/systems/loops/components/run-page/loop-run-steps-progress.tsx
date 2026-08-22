import { Gauge } from "lucide-react";

import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import type { LoopStepsProgressModel } from "../../lib/loop-run-progress";
import { type LoopRosterReach, loopRosterReachNote } from "../../lib/loop-run-registers-view";
import { LoopSection } from "../loop-section";
import { LOOP_PROGRESS_SEGMENT_CLASS } from "./loop-progress-segment-class";
import { LoopRunStepRow } from "./loop-run-step-row";

interface LoopRunStepsProgressProps {
  progress: LoopStepsProgressModel;
  /** The loop's stated goal; falls back to the run identity when no contract loads. */
  goal?: string;
  doneWhen?: string;
  /** How much of the roster the step list below was built from. */
  reach?: LoopRosterReach;
}

/**
 * How far along the run is, in steps and rounds.
 *
 * Two channels, and they do not overlap. The bar is magnitude: it says how much
 * of the round has settled and nothing else, which is why a parked segment keeps
 * the informational ink instead of shouting. The alarm-bearing state lives one
 * line below on each step's chip, where tone, glyph and word travel together.
 * Colouring both would spend two channels on one fact.
 */
export function LoopRunStepsProgress({
  progress,
  goal,
  doneWhen,
  reach,
}: LoopRunStepsProgressProps) {
  // The served counts above stay exact whatever the roster read (SI-12). The step
  // list below is built from the roster itself, so a partial read makes it short —
  // and it has to say so rather than read as the whole round.
  const reachNote = reach ? loopRosterReachNote(reach) : null;
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-progress"
      gist={progress.label}
      icon={<Gauge aria-hidden="true" />}
      title="Progress"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <div className="px-4.5 pt-4 pb-4.25">
          <h2
            className="text-item-title font-medium tracking-tight text-pretty text-fg-strong"
            data-testid="loop-run-progress-label"
          >
            {goal ?? progress.label}
          </h2>
          {doneWhen ? (
            <p
              className="mt-1 max-w-[62ch] text-small-body leading-relaxed text-muted"
              data-testid="loop-run-done-when"
            >
              {doneWhen}
            </p>
          ) : null}
          {progress.segments.length > 0 ? (
            <div
              aria-label={progress.ariaLabel}
              className="mt-3.75 flex h-1.5 gap-0.75"
              data-testid="loop-run-progress-bar"
              role="img"
            >
              {withOccurrenceKeys(progress.segments, segment => segment).map(
                ({ item: segment, key }) => (
                  <span
                    className={`flex-1 rounded-pill ${LOOP_PROGRESS_SEGMENT_CLASS[segment]}`}
                    data-state={segment}
                    key={key}
                  />
                )
              )}
            </div>
          ) : null}
          <div className="mt-2.25 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-form-label text-muted">
            <span data-testid="loop-run-progress-meta">{progress.leftMeta}</span>
            {progress.rightMeta ? (
              <span className="whitespace-nowrap text-subtle" data-testid="loop-run-progress-right">
                {progress.rightMeta}
              </span>
            ) : null}
          </div>
          {progress.steps.length > 0 ? (
            <ul className="mt-3.5 flex flex-col" data-testid="loop-run-step-list">
              {progress.steps.map(step => (
                <LoopRunStepRow key={step.key} step={step} />
              ))}
            </ul>
          ) : null}
          {reachNote ? (
            <p className="mt-3 text-form-hint text-subtle" data-testid="loop-run-progress-reach">
              {reachNote}
            </p>
          ) : null}
        </div>
      </div>
    </LoopSection>
  );
}
