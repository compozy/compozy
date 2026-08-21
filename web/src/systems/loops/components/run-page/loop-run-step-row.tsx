import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import type { LoopFanOutBand } from "../../lib/loop-run-fanout-band";
import type { LoopStepRow } from "../../lib/loop-run-progress";
import { LOOP_PROGRESS_SEGMENT_CLASS } from "./loop-progress-segment-class";
import { LoopNodeStateChip } from "./loop-node-state-chip";

interface LoopRunStepRowProps {
  step: LoopStepRow;
}

/**
 * One step of the round — and, when the step fanned out, its whole width.
 *
 * A fan-out is one row, never a set of siblings. But collapsing it to "2/3"
 * would hide which worker is holding things up, so the band draws a lane per
 * worker in the same segment vocabulary the bar above uses, and names them while
 * there are few enough to read. Past that it keeps the lanes and says the rest
 * in a sentence: width and fate stay visible at any scale.
 */
function LoopStepFanOutBand({ band }: { band: LoopFanOutBand }) {
  return (
    <div className="mt-1.5 border-l border-line-soft pl-2.5" data-testid="loop-run-step-fanout">
      <div
        aria-label={band.summary}
        className="flex h-1 gap-0.5"
        data-testid="loop-run-step-fanout-lanes"
        role="img"
      >
        {withOccurrenceKeys(band.segments, segment => segment).map(({ item: segment, key }) => (
          <span
            className={`flex-1 rounded-pill ${LOOP_PROGRESS_SEGMENT_CLASS[segment]}`}
            data-state={segment}
            key={key}
          />
        ))}
      </div>
      {band.wide ? (
        <p className="mt-1.5 text-form-hint text-subtle" data-testid="loop-run-step-fanout-summary">
          {band.summary}
        </p>
      ) : (
        <ul className="mt-1.5 flex flex-col gap-1">
          {band.branches.map(branch => (
            <li className="flex items-center gap-2 text-form-hint" key={branch.key}>
              <span className="min-w-0 truncate text-subtle">{branch.label}</span>
              {branch.attemptLabel ? (
                <span className="shrink-0 font-mono text-mono-id text-faint">
                  {branch.attemptLabel}
                </span>
              ) : null}
              <span className="ml-auto shrink-0">
                <LoopNodeStateChip chip={branch.chip} />
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function LoopRunStepRow({ step }: LoopRunStepRowProps) {
  return (
    <li
      className="flex items-start gap-2.5 border-t border-line-soft py-2.25 first:border-t-0"
      data-control={step.isControl ? "true" : undefined}
      data-node-id={step.nodeId}
      data-testid={`loop-run-step-${step.nodeId}`}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="min-w-0 truncate text-small-body font-medium text-fg-strong">
            {step.nodeId}
          </span>
          {step.attemptLabel ? (
            // Attempts are metadata on the step. They never become steps.
            <span className="shrink-0 font-mono text-mono-id text-faint">{step.attemptLabel}</span>
          ) : null}
        </div>
        {step.fanOut ? <LoopStepFanOutBand band={step.fanOut} /> : null}
      </div>
      <span className="shrink-0">
        {step.fanOut ? (
          <span
            className="inline-flex items-center gap-1.5"
            data-testid={`loop-run-step-rollup-${step.nodeId}`}
          >
            <span className="font-mono text-mono-id text-subtle">{step.fanOut.countLabel}</span>
            <LoopNodeStateChip chip={step.chip} />
          </span>
        ) : (
          <LoopNodeStateChip chip={step.chip} />
        )}
      </span>
    </li>
  );
}
