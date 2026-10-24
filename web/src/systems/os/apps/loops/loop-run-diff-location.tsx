import { AlertCircle, GitCompare } from "lucide-react";

import { Empty, ListingPage, useTopbarSlot } from "@compozy/ui";

import { loopRunsTrail } from "./loop-window-crumbs";
import { useLoopRunDiffPage } from "./use-loop-run-diff-page";

import {
  LoopRunDiffPageSkeleton,
  LoopRunDiffPickers,
  LoopRunDiffView,
  type LoopRunDiffRouteSearch,
  LoopStatusMark,
  loopRunDiffQuery,
} from "@/systems/loops";
import { useLoopWindowScope } from "./hooks/use-loop-window-scope";
import { LoopUnboundState } from "./loop-unbound-state";

export function LoopRunDiffLocation({
  runId,
  search,
}: {
  runId: string;
  search: LoopRunDiffRouteSearch;
}) {
  const { scope, workspaceId } = useLoopWindowScope();
  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="loop-run-diff-location">
      {workspaceId === "" ? (
        <LoopUnboundState
          description="Open this run from Runs to compare it."
          scope={scope}
          testId="loop-run-diff-no-workspace"
        />
      ) : (
        <LoopRunDiffPage key={runId} runId={runId} search={search} workspaceId={workspaceId} />
      )}
    </div>
  );
}

interface LoopRunDiffPageProps {
  runId: string;
  search: LoopRunDiffRouteSearch;
  workspaceId: string;
}

function LoopRunDiffPage({ runId, search, workspaceId }: LoopRunDiffPageProps) {
  const page = useLoopRunDiffPage(workspaceId, runId, search);
  useTopbarSlot({
    ...loopRunsTrail({
      level: "compare",
      loopName: page.loopName === "" ? undefined : page.loopName,
      onBack: page.goToRun,
      openLoop: page.loopName === "" ? undefined : page.goToLoop,
      openLoops: page.goToLoops,
      openRun: page.goToRun,
      openRuns: page.goToRuns,
      runId,
    }),
    status: page.run ? <LoopStatusMark status={page.run.status} /> : undefined,
  });

  if (page.isRunLoading) {
    return (
      <ListingPage>
        <LoopRunDiffPageSkeleton hasComparison={loopRunDiffQuery(search) !== null} />
      </ListingPage>
    );
  }

  if (page.runError || !page.run) {
    return (
      <DiffState
        description={page.runError?.message ?? `Run ${runId} not found.`}
        icon={AlertCircle}
        role="alert"
        testId="loop-run-diff-not-found"
        title="Unable to load run"
      />
    );
  }

  return (
    <ListingPage data-testid="loop-run-diff-page">
      <LoopRunDiffView
        error={page.diffError}
        isLoading={page.isDiffLoading}
        pickers={
          <LoopRunDiffPickers
            againstGeneration={page.againstGeneration}
            againstRunId={page.againstRunId}
            againstStatus={page.diffView?.terminalAgainst}
            baseGeneration={page.baseGeneration}
            baseStatus={page.diffView?.terminalBase}
            generations={page.generations}
            mode={page.mode}
            onAgainstGenerationChange={page.onAgainstGenerationChange}
            onAgainstRunChange={page.onAgainstRunChange}
            onBaseGenerationChange={page.onBaseGenerationChange}
            onModeChange={page.onModeChange}
            runs={page.runOptions}
          />
        }
        view={page.diffView}
      />
      {page.hasComparison ? null : (
        <Empty
          className="mt-4"
          data-testid="loop-run-diff-unselected"
          description="Pick what to compare with: another round of this run, or another run of this Loop."
          framed
          icon={GitCompare}
          title="Nothing compared yet"
          titleAs="h3"
        />
      )}
    </ListingPage>
  );
}

interface DiffStateProps {
  description: string;
  icon?: typeof GitCompare;
  role?: "alert";
  testId: string;
  title: string;
}

function DiffState({ description, icon = GitCompare, role, testId, title }: DiffStateProps) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center py-10"
      data-testid={testId}
      role={role}
    >
      <Empty className="max-w-md" description={description} icon={icon} title={title} />
    </div>
  );
}
