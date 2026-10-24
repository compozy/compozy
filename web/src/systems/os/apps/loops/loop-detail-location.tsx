import { AlertCircle, Repeat2 } from "lucide-react";

import { Empty } from "@compozy/ui";

import { LoopUnboundState } from "./loop-unbound-state";
import { useLoopDetail } from "./use-loop-detail";
import { LoopDetailSkeleton, LoopDetailView } from "@/systems/loops";

export function LoopDetailLocation({ name }: { name: string }) {
  const {
    scope,
    workspaceId,
    loopQuery,
    configQuery,
    catalogEntry,
    runsQuery,
    bindings,
    deleteLoop,
    readGraph,
    handlers,
  } = useLoopDetail(name);
  if (workspaceId === "") {
    return (
      <LoopUnboundState
        description="Open this Loop from the Loops catalog."
        scope={scope}
        testId="loop-detail-no-workspace"
      />
    );
  }
  // "No data yet" rather than "fetching": a window outside the live-data budget has not
  // started its reads, and must not read as a missing Loop.
  if (loopQuery.isPending || configQuery.isPending) {
    return <LoopDetailSkeleton name={name} />;
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
      recentRunsLoading={runsQuery.isPending}
      bindings={bindings.rows}
      bindingsLoading={bindings.isPending}
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
      onAutomate={handlers.onAutomate}
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
