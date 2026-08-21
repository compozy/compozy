import { Button, Empty, Pill, Time } from "@compozy/ui";

import type {
  LoopGenerationProgressState,
  LoopGenerationRow,
} from "../../../lib/loop-generation-presentation";
import { formatTokenCount } from "../../../lib/loop-runs-view";

interface LoopGenerationHistoryProps {
  rows: readonly LoopGenerationRow[];
  onCompare?: (generation: number) => void;
  onFork?: (generation: number) => void;
}

/**
 * How the run converged, round by round.
 *
 * Scores appear only where the loop defines scoring. A loop with no gate metric
 * gets no score at all rather than a dash, because an empty column invites the
 * reader to wonder what they are missing.
 *
 * A round interrupted by a crash keeps whatever it actually recorded. Filling in
 * a completion it never reached would be the single most misleading thing this
 * view could do.
 */
const PROGRESS_LABELS: Record<LoopGenerationProgressState, string | null> = {
  settled: null,
  "in-progress": "still running",
  partial: "partly settled",
  interrupted: "interrupted before it finished",
};

export function LoopGenerationHistory({ rows, onCompare, onFork }: LoopGenerationHistoryProps) {
  if (rows.length === 0) {
    return (
      <Empty
        data-testid="loop-generation-history-empty"
        description="This run has not completed a round yet, so there is no history to compare."
        title="No rounds yet"
      />
    );
  }
  return (
    <ul className="flex flex-col divide-y divide-line-soft" data-testid="loop-generation-history">
      {rows.map(row => {
        const progressLabel = PROGRESS_LABELS[row.progressState];
        return (
          <li
            className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-3"
            data-generation={row.generation}
            data-progress={row.progressState}
            data-testid={`loop-generation-${row.generation}`}
            key={row.generation}
          >
            <span className="font-mono text-mono-id text-subtle">Round {row.generation}</span>
            <Pill tone={row.tone}>{row.outcomeLabel}</Pill>
            {progressLabel ? (
              <span
                className="text-form-hint text-muted"
                data-testid={`loop-generation-progress-${row.generation}`}
              >
                {progressLabel}
              </span>
            ) : null}
            {/* Only a loop that defines scoring gets a score. */}
            {row.scoreLabel ? (
              <span
                className="font-mono text-mono-id text-subtle"
                data-testid={`loop-generation-score-${row.generation}`}
              >
                {row.scoreLabel}
              </span>
            ) : null}
            <span className="text-form-hint text-faint">{row.originLabel}</span>
            <span className="text-form-hint text-faint">
              {row.stepResults} {row.stepResults === 1 ? "step result" : "step results"}
            </span>
            {row.tokens !== null ? (
              <span
                className="font-mono text-mono-id text-subtle"
                data-testid={`loop-generation-usage-${row.generation}`}
              >
                {formatTokenCount(row.tokens)}
                {row.costLabel ? <span className="text-faint"> · {row.costLabel} est.</span> : null}
              </span>
            ) : null}
            {row.endedAt ? (
              <span className="text-form-hint text-faint">
                <Time iso={row.endedAt} />
              </span>
            ) : null}
            <span className="ml-auto flex items-center gap-1.5">
              {onCompare ? (
                <Button
                  data-testid={`loop-generation-compare-${row.generation}`}
                  onClick={() => onCompare(row.generation)}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  Compare
                </Button>
              ) : null}
              {onFork ? (
                <Button
                  data-testid={`loop-generation-fork-${row.generation}`}
                  onClick={() => onFork(row.generation)}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  Fork from here
                </Button>
              ) : null}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
