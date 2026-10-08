import { useProfileReadScope } from "@/systems/profiles";
import { useInfiniteQuery, useQueries, useQuery } from "@tanstack/react-query";

import { type LoopChildRunSummary, buildChildRunSummary } from "../lib/loop-run-child-runs";
import {
  loopRunBriefingOptions,
  loopRunDetailOptions,
  loopRunRosterOptions,
} from "../lib/query-options";

export interface LoopChildRunRead {
  summary: LoopChildRunSummary | null;
  isLoading: boolean;
  isError: boolean;
}

/**
 * One child run, read the way its own page reads it.
 *
 * The same detail, briefing and roster options the child's page uses, so
 * opening the child afterwards is a cache hit, and every read stops polling the
 * moment the child settles. The detail names the run and times it, the briefing
 * serves its step counts, and the roster says which step it is on. Only the
 * first roster page is read: the current step lives in the current round, and a
 * child wide enough to page past it still reports its counts on the briefing.
 */
export function useLoopChildRun(
  workspaceId: string,
  runId: string,
  nowMs: number
): LoopChildRunRead {
  const { params } = useProfileReadScope();
  const detail = useQuery(loopRunDetailOptions(workspaceId, runId, true, params));
  const briefing = useQuery(loopRunBriefingOptions(workspaceId, runId, true, params));
  const roster = useInfiniteQuery(loopRunRosterOptions(workspaceId, runId, {}, true, params));
  const run = detail.data?.run ?? null;
  const nodes = roster.data?.pages[0]?.nodes ?? [];
  return {
    summary: run
      ? buildChildRunSummary(
          run,
          briefing.data?.progress ?? null,
          nodes,
          nowMs,
          // A step parked in a durable wait is timed by its wait cell.
          detail.data?.waits ?? []
        )
      : null,
    isLoading: detail.isPending,
    isError: detail.isError,
  };
}

/**
 * The inputs each listed child was started with, for telling siblings apart.
 *
 * The same detail options the rows read, so the list and its rows share one
 * cache entry per child rather than reading each child twice.
 */
export function useLoopChildRunInputs(
  workspaceId: string,
  runIds: readonly string[]
): { runId: string; inputs: Record<string, unknown> | null }[] {
  const { params } = useProfileReadScope();
  return useQueries({
    queries: runIds.map(runId => loopRunDetailOptions(workspaceId, runId, true, params)),
    combine: results =>
      results.map((result, index) => ({
        runId: runIds[index],
        inputs: result.data?.run.inputs ?? null,
      })),
  });
}
