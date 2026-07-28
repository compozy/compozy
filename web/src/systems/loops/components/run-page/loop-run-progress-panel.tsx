import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import type { LoopProgressSegmentState, LoopRunProgressModel } from "../../lib/loop-run-progress";
import { LoopRunSection } from "./loop-run-section";

interface LoopRunProgressPanelProps {
  /** `contract.goal`, falling back to the run identity when no contract loads. */
  title: string;
  /** `contract.definition_of_done`; omitted when the contract has not loaded. */
  doneWhen?: string;
  progress: LoopRunProgressModel;
}

const SEGMENT_CLASS: Record<LoopProgressSegmentState, string> = {
  clean: "bg-success",
  active: "bg-accent-dim",
  redo: "bg-badge-fill",
  failed: "bg-danger",
  idle: "bg-badge-fill",
};

/**
 * The Progress panel (§2/§5.2): the goal as the page's title voice, the
 * done-when line, and the equal-width group bar with its plain meta line.
 */
export function LoopRunProgressPanel({ title, doneWhen, progress }: LoopRunProgressPanelProps) {
  return (
    <LoopRunSection label="Progress" data-testid="loop-run-progress">
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <div className="px-4.5 pt-4 pb-4.25">
          <h2
            className="text-item-title font-medium tracking-tight text-pretty text-fg-strong"
            data-testid="loop-run-goal"
          >
            {title}
          </h2>
          {doneWhen ? (
            <p
              className="mt-1 max-w-[62ch] text-small-body leading-relaxed text-muted"
              data-testid="loop-run-done-when"
            >
              {doneWhen}
            </p>
          ) : null}
          {progress.segments ? (
            <div
              className="mt-3.75 flex h-1.5 gap-0.75"
              role="img"
              aria-label={progress.ariaLabel}
              data-testid="loop-run-progress-bar"
            >
              {withOccurrenceKeys(progress.segments, state => state).map(({ item: state, key }) => (
                <span
                  key={key}
                  data-state={state}
                  className={`flex-1 rounded-pill ${SEGMENT_CLASS[state]}`}
                />
              ))}
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
        </div>
      </div>
    </LoopRunSection>
  );
}
