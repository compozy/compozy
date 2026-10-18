import { useProfileReadScope } from "@/systems/profiles";
import { useEffect } from "react";
import { useInfiniteQuery, useQueries, useQuery } from "@tanstack/react-query";

import { isLiveLoopRun } from "../lib/loop-formatters";
import { type LoopChildRunSummary, buildChildRunSummary } from "../lib/loop-run-child-runs";
import {
  loopRunBriefingOptions,
  loopRunDetailOptions,
  loopRunRosterOptions,
} from "../lib/query-options";
import { useLoopRunChildRead } from "./use-loop-run-child-read";
import { useNowTick } from "./use-now-tick";

/**
 * How many pages of the child's current round a row reads before it stops and
 * says so. 200 rows a page: a round wider than this is a run to open, not a row.
 */
const CHILD_ROSTER_PAGE_CAP = 5;

export interface LoopChildRunRead {
  summary: LoopChildRunSummary | null;
  /** Any of the three reads still has nothing to show. */
  isLoading: boolean;
  /** Any of the three reads failed; whatever did arrive stays on screen. */
  isError: boolean;
  /** The current round is wider than the row reads, so its step may be elsewhere. */
  rosterTruncated: boolean;
}

/**
 * One child run, read the way its own page reads it.
 *
 * The detail names the run, times it and holds its wait cells; the briefing
 * serves its step counts and current round; the roster — that round only, every
 * page up to a cap — says which step it is on. All three use the child page's
 * own query options, so they stop polling the moment the child settles.
 */
export function useLoopChildRun(runId: string): LoopChildRunRead {
  const { workspaceId, nowMs: pageNowMs, clockLive } = useLoopRunChildRead();
  const { params } = useProfileReadScope();
  const detail = useQuery(loopRunDetailOptions(workspaceId, runId, true, params));
  const briefing = useQuery(loopRunBriefingOptions(workspaceId, runId, true, params));
  // The daemon pages the roster oldest round first, so the round a child is on
  // can sit far past page one. Asking for that round alone reads only what the
  // row needs; a child with no round yet has no step to find.
  const round = briefing.data?.progress.round ?? 0;
  const roster = useInfiniteQuery(
    loopRunRosterOptions(workspaceId, runId, { generation: round }, round > 0, params)
  );
  const pages = roster.data?.pages ?? [];
  const pageCount = pages.length;
  const atCap = pageCount >= CHILD_ROSTER_PAGE_CAP;
  const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = roster;
  // Keyed on the page count too: a page can arrive and the fetch flag settle
  // within one render, and the next page still has to be asked for. A page that
  // failed is not asked for again here — the next poll retries it, so a
  // struggling daemon is not met with a tight loop of requests.
  useEffect(() => {
    if (!hasNextPage || isFetchingNextPage || isFetchNextPageError) return;
    if (pageCount >= CHILD_ROSTER_PAGE_CAP) return;
    void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, isFetchNextPageError, pageCount, fetchNextPage]);

  const run = detail.data?.run ?? null;
  // The page clock stops with the parent; a child still running keeps ticking.
  const ownNowMs = useNowTick(!clockLive && isLiveLoopRun(run));
  const nowMs = clockLive ? pageNowMs : ownNowMs;
  return {
    summary: run
      ? buildChildRunSummary(
          run,
          briefing.data?.progress ?? null,
          pages.flatMap(page => page.nodes),
          nowMs,
          // A step parked in a durable wait is timed by its wait cell.
          detail.data?.waits ?? []
        )
      : null,
    isLoading: detail.isPending || briefing.isPending || (round > 0 && roster.isPending),
    isError: detail.isError || briefing.isError || roster.isError,
    rosterTruncated: atCap && hasNextPage,
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
