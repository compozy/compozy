import { AlertCircle, GitCompare } from "lucide-react";

import { Empty, ListingPage, Spinner, useTopbarSlot } from "@compozy/ui";

import { loopRunsTrail } from "./loop-window-crumbs";
import { useLoopRunDiffPage } from "./use-loop-run-diff-page";

import {
  LoopRunDiffPickers,
  LoopRunDiffView,
  type LoopRunDiffRouteSearch,
  LoopStatusPill,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export function LoopRunDiffLocation({
  runId,
  routeWorkspaceId,
  search,
}: {
  runId: string;
  routeWorkspaceId?: string;
  search: LoopRunDiffRouteSearch;
}) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = routeWorkspaceId ?? runtimeWorkspaceId ?? "";
  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="loop-run-diff-location">
      {workspaceId === "" ? (
        <DiffState
          description="Select a workspace to compare this run."
          testId="loop-run-diff-no-workspace"
          title="No workspace selected"
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
    status: page.run ? <LoopStatusPill status={page.run.status} /> : undefined,
  });

  if (page.isRunLoading) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center"
        data-testid="loop-run-diff-loading"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
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
          description="Pick an against side: another generation of this run, or another run of this loop."
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
