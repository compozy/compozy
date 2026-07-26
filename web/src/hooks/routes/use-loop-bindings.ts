import { useAutomationJobs, useAutomationTriggers } from "@/systems/automation";
import type { LoopBindingRow } from "@/systems/loops";

import { buildLoopBindingIndex } from "./loop-bindings-map";

const BINDING_PAGE_LIMIT = 50;

const EMPTY_ROWS: readonly LoopBindingRow[] = [];

export interface LoopBindingsResult {
  rows: readonly LoopBindingRow[];
  isLoading: boolean;
  jobs: LoopBindingPageState;
  triggers: LoopBindingPageState;
}

export interface LoopBindingPageState {
  error: Error | null;
  hasMore: boolean;
  isFetchingMore: boolean;
  loadMore: () => void;
  loaded: number;
  total: number;
}

/**
 * The attached loop-target automations for one Loop, using the `loop=<name>`
 * automation list filter (detail Start-bindings panel).
 */
export function useLoopBindings(workspaceId: string, loopName: string): LoopBindingsResult {
  const enabled = workspaceId !== "" && loopName !== "";
  const triggersQuery = useAutomationTriggers(
    {
      loop: loopName,
      limit: BINDING_PAGE_LIMIT,
      scope: "workspace",
      workspace_id: workspaceId,
    },
    { enabled }
  );
  const jobsQuery = useAutomationJobs(
    {
      loop: loopName,
      limit: BINDING_PAGE_LIMIT,
      scope: "workspace",
      workspace_id: workspaceId,
    },
    { enabled }
  );
  const triggers = triggersQuery.triggers;
  const jobs = jobsQuery.jobs;
  const rows =
    workspaceId && loopName
      ? (buildLoopBindingIndex(triggers, jobs, workspaceId).get(loopName)?.rows ?? EMPTY_ROWS)
      : EMPTY_ROWS;
  return {
    rows,
    isLoading: triggersQuery.isLoading || jobsQuery.isLoading,
    jobs: {
      error: jobsQuery.error,
      hasMore: Boolean(jobsQuery.hasNextPage),
      isFetchingMore: jobsQuery.isFetchingNextPage,
      loadMore: () => void jobsQuery.fetchNextPage(),
      loaded: jobs.length,
      total: jobsQuery.total,
    },
    triggers: {
      error: triggersQuery.error,
      hasMore: Boolean(triggersQuery.hasNextPage),
      isFetchingMore: triggersQuery.isFetchingNextPage,
      loadMore: () => void triggersQuery.fetchNextPage(),
      loaded: triggers.length,
      total: triggersQuery.total,
    },
  };
}
