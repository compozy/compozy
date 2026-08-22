import { Activity } from "lucide-react";

import { Empty } from "@compozy/ui";

import { buildRunKpis, partitionRuns, type LoopOutcomeValue } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopRunsKpis } from "./loop-runs-kpis";
import { LoopRunsTable } from "./loop-runs-table";
import { emptyForScope, type ProfileListingScope } from "@/systems/profiles";

interface LoopRunsViewProps {
  runs: readonly LoopRun[];
  /** Owner tags in aggregate mode; names the profile when nothing has run. */
  profile: ProfileListingScope;
  /** Outcome filter driven by the toolbar chip bar. */
  outcome: LoopOutcomeValue;
  pendingRequestCounts?: ReadonlyMap<string, number>;
}

export function LoopRunsView({ runs, outcome, profile, pendingRequestCounts }: LoopRunsViewProps) {
  const ownerOf = profile.aggregate ? profile.ownerOf : undefined;
  const kpis = buildRunKpis(runs);
  const { active, past } = partitionRuns(runs, outcome);
  const nothingMatches = active.length === 0 && past.length === 0;
  return (
    <div className="flex flex-col gap-5" data-testid="loop-runs-view">
      <LoopRunsKpis kpis={kpis} />
      {nothingMatches ? (
        <Empty
          className="mx-auto my-8 max-w-md"
          description={
            runs.length === 0
              ? profile.scopeLabel === null
                ? "Start a Loop and its run will belong to whichever profile started it."
                : "Start a Loop and its runs will belong to this profile."
              : "No runs match the selected outcome filter."
          }
          icon={Activity}
          title={runs.length === 0 ? emptyForScope("runs", profile.scopeLabel) : "No matching runs"}
        />
      ) : (
        <div className="flex flex-col gap-5">
          <LoopRunsTable
            ownerOf={ownerOf}
            pendingRequestCounts={pendingRequestCounts}
            runs={active}
            testId="loop-runs-active"
            title="Active"
          />
          <LoopRunsTable
            ownerOf={ownerOf}
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
