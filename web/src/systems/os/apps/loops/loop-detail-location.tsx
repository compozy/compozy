import { AlertCircle, Repeat2 } from "lucide-react";

import { Empty, PAGE_CONTENT_GUTTER, Skeleton, SkeletonRows, cn } from "@compozy/ui";

import { useLoopDetail } from "./use-loop-detail";
import { LoopDetailView } from "@/systems/loops";

export function LoopDetailLocation({
  name,
  routeWorkspaceId,
}: {
  name: string;
  routeWorkspaceId?: string;
}) {
  const {
    workspaceId,
    loopQuery,
    configQuery,
    catalogEntry,
    runsQuery,
    bindings,
    deleteLoop,
    readGraph,
    handlers,
  } = useLoopDetail(name, routeWorkspaceId);
  if (workspaceId === "") {
    return (
      <DetailState
        description="Select a project to inspect this Loop."
        testId="loop-detail-no-workspace"
        title="No project selected"
      />
    );
  }
  if (loopQuery.isLoading || configQuery.isLoading) {
    return (
      <div
        aria-busy="true"
        className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col gap-6 pt-6")}
        data-testid="loop-detail-loading"
      >
        <Skeleton className="h-7 w-48" />
        <SkeletonRows count={3} rowClassName="border-b border-line-soft py-3" />
      </div>
    );
  }
  if (loopQuery.error || configQuery.error || !loopQuery.data) {
    return (
      <DetailState
        description={
          loopQuery.error?.message ?? configQuery.error?.message ?? `Loop ${name} not found.`
        }
        icon={AlertCircle}
        testId="loop-detail-not-found"
        title="Couldn't open this Loop"
      />
    );
  }

  if (!configQuery.effectiveConfig) {
    return (
      <DetailState
        description="Couldn't load this Loop's settings. Try again in a moment."
        icon={AlertCircle}
        testId="loop-detail-config-error"
        title="Couldn't load settings"
      />
    );
  }

  const loop = loopQuery.data;
  return (
    <LoopDetailView
      loop={loop}
      effectiveConfig={configQuery.effectiveConfig}
      graph={readGraph(loop.definition)}
      recentRuns={runsQuery.data?.runs ?? []}
      bindings={bindings.rows}
      bindingsLoading={bindings.isLoading}
      bindingJobs={bindings.jobs}
      bindingTriggers={bindings.triggers}
      successRate={catalogEntry?.success_rate_30d ?? null}
      aggregate={catalogEntry?.aggregate_30d ?? null}
      onRun={handlers.onRun}
      onConfigure={handlers.onConfigure}
      onOpenEditor={handlers.onOpenEditor}
      onDelete={handlers.onDelete}
      onDeleteReset={handlers.onDeleteReset}
      deletePending={deleteLoop.isPending}
      deleteError={deleteLoop.error?.message ?? null}
      onAddTrigger={handlers.onAddTrigger}
      onAddSchedule={handlers.onAddSchedule}
    />
  );
}

interface DetailStateProps {
  title: string;
  description: string;
  testId: string;
  icon?: typeof Repeat2;
}

function DetailState({ title, description, testId, icon = Repeat2 }: DetailStateProps) {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center py-10" data-testid={testId}>
      <Empty className="max-w-md" description={description} icon={icon} title={title} />
    </div>
  );
}
