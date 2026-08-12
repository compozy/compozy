import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { LoopRunsRouteSearch } from "@/systems/loops";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  type LoopNodeInventorySort,
  type LoopOutcomeValue,
  useLoopNodeInventory,
  useLoopRuns,
  useLoops,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export function useLoopRunsRoute(search: LoopRunsRouteSearch) {
  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const workspaceLabel = activeWorkspace?.name ?? activeWorkspace?.id ?? "workspace";
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const enabled = workspaceId !== "" && liveDataEnabled;
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  // Sort is presentation over the loaded page, not a server contract — it stays
  // component state rather than a search param the API would have to honor.
  const [inventorySort, setInventorySort] = useState<LoopNodeInventorySort>("oldest");
  const navigate = useNavigate({ from: "/loop-runs" });
  const runsQuery = useLoopRuns(
    workspaceId,
    { origin: search.origin, origin_session: search.origin_session },
    enabled
  );

  const setOrigin = (origin: LoopRunsRouteSearch["origin"]) => {
    void navigate({
      to: "/loop-runs",
      search: current => ({
        ...current,
        origin,
        origin_session: origin === "session" ? current.origin_session : undefined,
      }),
    });
  };

  const setOriginSession = (originSession: string) => {
    void navigate({
      to: "/loop-runs",
      search: current => ({
        ...current,
        origin: "session",
        origin_session: originSession || undefined,
      }),
    });
  };

  const inventoryState = search.nodes;
  const catalogQuery = useLoops(
    workspaceId,
    { limit: 50, sort: "name" },
    enabled && inventoryState !== undefined
  );
  const loopOptions = [...new Set(catalogQuery.loops.map(loop => loop.name))].sort();
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
      to: "/loop-runs",
      search: current => ({ ...current, nodes: next }),
    });
  };

  const setInventoryLoop = (loop: string) => {
    void navigate({
      to: "/loop-runs",
      search: current => ({ ...current, nodes_loop: loop || undefined }),
    });
  };

  const clearInventoryFilters = () => {
    void navigate({
      to: "/loop-runs",
      search: current => ({ ...current, nodes_loop: undefined, nodes_run: undefined }),
    });
  };

  return {
    outcome,
    runsQuery,
    setOrigin,
    setOriginSession,
    setOutcome,
    workspaceId,
    workspaceLabel,
    inventoryState,
    inventory,
    loopOptions,
    inventorySort,
    setInventorySort,
    setInventoryState,
    setInventoryLoop,
    clearInventoryFilters,
  };
}

export type { LoopRunsRouteSearch } from "@/systems/loops";
