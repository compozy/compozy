import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import {
  advanceTimelineSnapshot,
  emptyTimelineSnapshot,
  loopTimelineSnapshotKey,
} from "../lib/loop-run-live-seam";
import {
  loopRunBriefingOptions,
  loopRunRosterOptions,
  loopRunTimelineOptions,
} from "../lib/query-options";
import type { LoopBriefing, LoopFanoutRollup, LoopRosterNode, LoopTimelineEntry } from "../types";

/**
 * The three run reads, bound for the page.
 *
 * Each is a separate cache entry because each has its own cadence: the briefing
 * and the roster poll while the run is live and stop when it settles, while the
 * timeline is driven by the stream instead of a timer. Flattening happens here,
 * at the view-model boundary — the envelopes, cursors and the `head_seq` fence
 * stay in the query cache where they belong.
 */

export interface LoopRunBriefingView {
  briefing: LoopBriefing | null;
  isLoading: boolean;
  isError: boolean;
}

export function useLoopRunBriefing(
  workspaceId: string,
  runId: string,
  enabled = true
): LoopRunBriefingView {
  const query = useQuery(loopRunBriefingOptions(workspaceId, runId, enabled));
  return {
    briefing: query.data ?? null,
    isLoading: query.isPending,
    isError: query.isError,
  };
}

/**
 * How many roster pages the page pulls on its own before it stops and says so.
 * 200 rows a page, so this converges any run under 2,000 node x round rows
 * without asking. Past it the operator asks explicitly — the page never claims
 * to be showing a run it only partly read.
 */
const ROSTER_PAGE_BUDGET = 10;

interface RosterPageAllowance {
  // Compared field by field rather than as one joined string: any separator can
  // also occur inside an id, and a collision here hands one run the page budget
  // a reader raised for another.
  workspaceId: string;
  runId: string;
  pages: number;
}

export interface LoopRunRosterView {
  nodes: LoopRosterNode[];
  rollups: LoopFanoutRollup[];
  loopName: string;
  runStatus: string;
  isLoading: boolean;
  isError: boolean;
  /** False while pages are still arriving — the views must not claim complete. */
  isComplete: boolean;
  /** True when the run is larger than the page pulled. Stated, never hidden. */
  isTruncated: boolean;
  /** Pulls the next block past the budget; the only way past it is asking. */
  loadMore: () => void;
  isLoadingMore: boolean;
}

export function useLoopRunRoster(
  workspaceId: string,
  runId: string,
  enabled = true
): LoopRunRosterView {
  const { data, hasNextPage, isFetchingNextPage, fetchNextPage, isPending, isError } =
    useInfiniteQuery(loopRunRosterOptions(workspaceId, runId, {}, enabled));
  const pages = data?.pages ?? [];
  const [allowance, setAllowance] = useState<RosterPageAllowance>(() => ({
    workspaceId,
    runId,
    pages: ROSTER_PAGE_BUDGET,
  }));
  // A different run starts over on the default budget rather than inheriting
  // whatever the previous run's reader asked for.
  const currentAllowance =
    allowance.workspaceId === workspaceId && allowance.runId === runId
      ? allowance
      : { workspaceId, runId, pages: ROSTER_PAGE_BUDGET };
  if (currentAllowance !== allowance) setAllowance(currentAllowance);
  const atPageCap = pages.length >= currentAllowance.pages;

  // The roster is only the truth if it is the WHOLE roster. The daemon returns
  // oldest generation first, so a run stopped at page one can be missing the
  // round the page is actually about — the DAG would then draw a current round
  // that is simply absent. Converging here is not an optimisation; it is what
  // makes "every node x round" a true statement.
  useEffect(() => {
    if (!hasNextPage || isFetchingNextPage || atPageCap) return;
    void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, atPageCap, fetchNextPage]);

  const nodes = pages.flatMap(page => page.nodes);
  // Rollups repeat on every page by design, so an agent never has to walk the
  // items to learn the counts. Dedupe by the node they roll up.
  const rollupsByKey = new Map<string, LoopFanoutRollup>();
  for (const page of pages) {
    for (const rollup of page.fanout_rollups) {
      rollupsByKey.set(`${rollup.generation}:${rollup.node_id}`, rollup);
    }
  }
  const rollups = [...rollupsByKey.values()];
  return {
    nodes,
    rollups,
    loopName: pages[0]?.loop_name ?? "",
    runStatus: pages.at(-1)?.run_status ?? "",
    isLoading: isPending,
    isError,
    isComplete: !isPending && !hasNextPage,
    isTruncated: atPageCap && hasNextPage,
    loadMore: () =>
      setAllowance(current => ({
        workspaceId,
        runId,
        pages: current.pages + ROSTER_PAGE_BUDGET,
      })),
    isLoadingMore: isFetchingNextPage,
  };
}

export interface LoopRunTimelineView {
  entries: readonly LoopTimelineEntry[];
  /** The sequence the stream resumes after, latched at the first known head. */
  headSeq: number | null;
  hasOlder: boolean;
  isLoading: boolean;
  isError: boolean;
  loadOlder: () => void;
  isLoadingOlder: boolean;
}

export function useLoopRunTimeline(
  workspaceId: string,
  runId: string,
  view: "notable" | "all" = "notable",
  enabled = true
): LoopRunTimelineView {
  const { data, hasNextPage, isFetchingNextPage, fetchNextPage, isPending, isError } =
    useInfiniteQuery(loopRunTimelineOptions(workspaceId, runId, { view }, enabled));
  const snapshotKey = loopTimelineSnapshotKey(workspaceId, runId, view);
  const [snapshot, setSnapshot] = useState(() => emptyTimelineSnapshot(snapshotKey));
  // A lifecycle frame invalidates these reads, and a refetch re-reads the newest
  // window unpinned before re-deriving every backward cursor from it — so the
  // loaded window slides up and the oldest pages drop off the bottom. Rendering
  // `pages` directly would take history away from whoever paged back to read it.
  // The snapshot is the union across every read, de-duped by sequence.
  const currentSnapshot = advanceTimelineSnapshot(snapshot, snapshotKey, data?.pages ?? []);
  if (currentSnapshot !== snapshot) setSnapshot(currentSnapshot);

  return {
    entries: currentSnapshot.entries,
    headSeq: currentSnapshot.fence,
    hasOlder: hasNextPage,
    isLoading: isPending,
    isError,
    loadOlder: () => {
      if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
    },
    isLoadingOlder: isFetchingNextPage,
  };
}
