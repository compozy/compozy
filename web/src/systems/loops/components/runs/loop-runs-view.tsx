import { Activity } from "lucide-react";

import { Empty } from "@compozy/ui";

import { buildRunKpis, partitionRuns, type LoopOutcomeValue } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopRunsKpis } from "./loop-runs-kpis";
import { LoopRunsTable } from "./loop-runs-table";

interface LoopRunsViewProps {
  runs: readonly LoopRun[];
  /** Outcome filter driven by the toolbar chip bar. */
  outcome: LoopOutcomeValue;
  pendingRequestCounts?: ReadonlyMap<string, number>;
}

export function LoopRunsView({ runs, outcome, pendingRequestCounts }: LoopRunsViewProps) {
  const kpis = buildRunKpis(runs);
  const { active, past } = partitionRuns(runs, outcome);
  const nothingMatches = active.length === 0 && past.length === 0;
  return (
    <div className="flex flex-col gap-5" data-testid="loop-runs-view">
      <LoopRunsKpis kpis={kpis} />
      {nothingMatches ? (
        <Empty
          className="mx-auto my-8 max-w-md"
          description="No runs match the selected outcome filter."
          icon={Activity}
          title="No matching runs"
        />
      ) : (
        <div className="flex flex-col gap-5">
          <LoopRunsTable
            pendingRequestCounts={pendingRequestCounts}
            runs={active}
            testId="loop-runs-active"
            title="Active"
          />
          <LoopRunsTable
            pendingRequestCounts={pendingRequestCounts}
            runs={past}
            testId="loop-runs-past"
            title="Past"
          />
        </div>
      )}
    </div>
  );
}
