import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import {
  isLoopNodeInventoryState,
  type LoopNodeInventorySort,
  type LoopNodeInventoryState,
  type LoopOutcomeValue,
  useLoopNodeInventory,
  useLoopRuns,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export interface LoopRunsRouteSearch {
  origin?: "catalog" | "session";
  origin_session?: string;
  /**
   * Selects the node inventory view instead of the run roster. Absent → runs.
   * It is a search param so a Waiting panel row can deep-link straight into the
   * matching inventory (`/loop-runs?nodes=waiting`).
   */
  nodes?: LoopNodeInventoryState;
  /** Inventory loop filter, passed straight through to `GET /loop-nodes`. */
  nodes_loop?: string;
  /** Inventory run filter, passed straight through to `GET /loop-nodes`. */
  nodes_run?: string;
}

export function useLoopRunsRoute(search: LoopRunsRouteSearch) {
  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const workspaceLabel = activeWorkspace?.name ?? activeWorkspace?.id ?? "workspace";
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  // Sort is presentation over the loaded page, not a server contract — it stays
  // component state rather than a search param the API would have to honor.
  const [inventorySort, setInventorySort] = useState<LoopNodeInventorySort>("oldest");
  const navigate = useNavigate({ from: "/loop-runs" });
  const runsQuery = useLoopRuns(
    workspaceId,
    { origin: search.origin, origin_session: search.origin_session },
    workspaceId !== ""
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
  const inventory = useLoopNodeInventory(
    workspaceId,
    {
      // `state` is required by the route; the query stays disabled until a view
      // is selected so the runs roster never fires an inventory request.
      state: inventoryState ?? "waiting",
      loop: search.nodes_loop || undefined,
      run_id: search.nodes_run || undefined,
    },
    workspaceId !== "" && inventoryState !== undefined
  );

  const setInventoryState = (next: LoopNodeInventoryState | undefined) => {
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
    inventorySort,
    setInventorySort,
    setInventoryState,
    setInventoryLoop,
    clearInventoryFilters,
  };
}

export function validateLoopRunsSearch(search: Record<string, unknown>): LoopRunsRouteSearch {
  const origin =
    search.origin === "catalog" || search.origin === "session" ? search.origin : undefined;
  const originSession =
    typeof search.origin_session === "string" ? search.origin_session.trim() : "";
  const nodes = isLoopNodeInventoryState(search.nodes) ? search.nodes : undefined;
  const nodesLoop = typeof search.nodes_loop === "string" ? search.nodes_loop.trim() : "";
  const nodesRun = typeof search.nodes_run === "string" ? search.nodes_run.trim() : "";
  return {
    origin,
    origin_session: originSession || undefined,
    nodes,
    // Inventory filters only mean something with a selected view; drop them
    // otherwise so the runs roster URL stays clean.
    nodes_loop: nodes === undefined ? undefined : nodesLoop || undefined,
    nodes_run: nodes === undefined ? undefined : nodesRun || undefined,
  };
}
