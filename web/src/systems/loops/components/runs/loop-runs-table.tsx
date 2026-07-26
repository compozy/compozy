import { Eyebrow } from "@agh/ui";

import type { LoopRun } from "../../types";
import { LOOP_RUNS_ROW_GRID, LoopRunRow } from "./loop-run-row";

interface LoopRunsTableProps {
  title: string;
  runs: readonly LoopRun[];
  testId: string;
}

// The run projection exposes only `created_at` (no `ended_at`), so the time column
// is always the start time — never "Ended" (that would be untruthful).
const COLUMNS: readonly string[] = ["Outcome", "Loop", "Inputs", "Gens", "Started", "Budget"];

/**
 * One runs table section (Active or Past) with a labeled header and run rows.
 * Renders nothing when the section is empty so the outcome filter hides sections
 * with no matching runs.
 */
export function LoopRunsTable({ title, runs, testId }: LoopRunsTableProps) {
  if (runs.length === 0) return null;
  return (
    <section data-testid={testId}>
      <div className="flex items-center gap-2 px-0.5 pb-2 pt-1">
        <Eyebrow className="text-subtle">{title}</Eyebrow>
        <span className="font-mono text-mono-id tabular-nums text-faint">{runs.length}</span>
      </div>
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <div
          className={`grid ${LOOP_RUNS_ROW_GRID} gap-3.5 border-b border-line-soft px-4 py-2.5 max-[1140px]:hidden`}
        >
          {COLUMNS.map(column => (
            <Eyebrow key={column} className="text-faint">
              {column}
            </Eyebrow>
          ))}
        </div>
        {runs.map(run => (
          <LoopRunRow key={run.id} run={run} />
        ))}
      </div>
    </section>
  );
}
