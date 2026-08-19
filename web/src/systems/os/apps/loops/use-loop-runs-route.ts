import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import type { LoopRunsRouteSearch } from "@/systems/loops";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  type LoopOutcomeValue,
  useLoopNodeInventory,
  useLoopRuns,
  useLoops,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export function useLoopRunsRoute(search: LoopRunsRouteSearch) {
  const { activeWorkspace, runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId ?? "";
  const workspaceLabel = activeWorkspace?.name ?? activeWorkspace?.id ?? "workspace";
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const enabled = workspaceId !== "" && liveDataEnabled;
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  const navigate = useNavigate({ from: "/loop-runs" });
  const runsQuery = useLoopRuns(
    workspaceId,
    { origin: search.origin, origin_session: search.origin_session },
    enabled
  );

  const setOriginFilter = ({
    origin,
    originSession,
  }: {
    origin?: LoopRunsRouteSearch["origin"];
    originSession?: string;
  }) => {
    if (search.origin === origin && search.origin_session === originSession) return;
    void navigate({
      to: "/loop-runs",
      search: current => ({ ...current, origin, origin_session: originSession }),
    });
  };

  const inventoryState = search.nodes;
  const catalogQuery = useLoops(
    workspaceId,
    { limit: 50, sort: "name" },
    enabled && inventoryState !== undefined
  );
  const loopOptions = [...new Set(catalogQuery.loops.map(loop => loop.name))].sort();
  const runOptions = [...new Set((runsQuery.data?.runs ?? []).map(run => run.id))];
  const inventory = useLoopNodeInventory(
    workspaceId,
    {
      // `state` is required by the route; the query stays disabled until a view
      // is selected so the runs roster never fires an inventory request.
      state: inventoryState ?? "waiting",
      loop: search.nodes_loop || undefined,
      run_id: search.nodes_run || undefined,
    },
    enabled && inventoryState !== undefined
  );

  const setInventoryState = (next: LoopRunsRouteSearch["nodes"]) => {
    void navigate({
      replace: true,
      to: "/loop-runs",
      search: current => ({ ...current, nodes: next }),
    });
  };

  const setInventoryLoop = (loop: string) => {
    void navigate({
      replace: true,
      to: "/loop-runs",
      search: current => ({ ...current, nodes_loop: loop || undefined }),
    });
  };

  const setInventoryRun = (runId: string) => {
    void navigate({
      replace: true,
      to: "/loop-runs",
      search: current => ({ ...current, nodes_run: runId || undefined }),
    });
  };

  const setInventoryView = (view: ListingViewMode) => {
    void navigate({
      replace: true,
      to: "/loop-runs",
      search: current => ({ ...current, view: view === "rows" ? undefined : view }),
    });
  };

  const clearInventoryFilters = () => {
    void navigate({
      replace: true,
      to: "/loop-runs",
      search: current => ({ ...current, nodes_loop: undefined, nodes_run: undefined }),
    });
  };

  return {
    outcome,
    runsQuery,
    setOriginFilter,
    setOutcome,
    workspaceId,
    workspaceLabel,
    inventoryState,
    inventory,
    loopOptions,
    runOptions,
    setInventoryState,
    setInventoryLoop,
    setInventoryRun,
    setInventoryView,
    clearInventoryFilters,
  };
}

export type { LoopRunsRouteSearch } from "@/systems/loops";
