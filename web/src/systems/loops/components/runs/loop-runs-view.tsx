import { Activity } from "lucide-react";

import { Empty } from "@agh/ui";

import { buildOutcomeSegments, buildRunKpis, partitionRuns } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopRunsKpis } from "./loop-runs-kpis";
import { LoopRunsOutcomeFilter, type LoopOutcomeValue } from "./loop-runs-outcome-filter";
import { LoopRunsTable } from "./loop-runs-table";

interface LoopRunsViewProps {
  runs: readonly LoopRun[];
  outcome: LoopOutcomeValue;
  onOutcomeChange: (value: LoopOutcomeValue) => void;
}

/**
 * The workspace-wide Runs surface (design §4.5): KPIs, a data-driven outcome
 * filter, and Active/Past tables. KPIs summarize every run; the tables reflect
 * the selected outcome and hide when empty.
 */
export function LoopRunsView({ runs, outcome, onOutcomeChange }: LoopRunsViewProps) {
  const kpis = buildRunKpis(runs);
  const segments = buildOutcomeSegments(runs);
  const { active, past } = partitionRuns(runs, outcome);
  const nothingMatches = active.length === 0 && past.length === 0;
  return (
    <div className="flex flex-col gap-5" data-testid="loop-runs-view">
      <LoopRunsKpis kpis={kpis} />
      <LoopRunsOutcomeFilter segments={segments} value={outcome} onChange={onOutcomeChange} />
      {nothingMatches ? (
        <Empty
          className="mx-auto my-8 max-w-md"
          description="No runs match the selected outcome filter."
          icon={Activity}
          title="No matching runs"
        />
      ) : (
        <div className="flex flex-col gap-5">
          <LoopRunsTable testId="loop-runs-active" title="Active" runs={active} />
          <LoopRunsTable testId="loop-runs-past" title="Past" runs={past} />
        </div>
      )}
    </div>
  );
}
