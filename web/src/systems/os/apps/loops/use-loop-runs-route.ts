import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import type { LoopRunsRouteSearch } from "@/systems/loops";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  type LoopOutcomeValue,
  useLoopNodeInventory,
  useLoopRunsList,
  useLoops,
} from "@/systems/loops";
import {
  breadthQuery,
  loopBreadthTarget,
  useActiveWorkspace,
  useProjectFilter,
  workspaceBadgeIdentity,
} from "@/systems/workspace";
import { useProfileReadScope } from "@/systems/profiles";
import { useLoopWindowScope } from "./hooks/use-loop-window-scope";

/**
 * View-model for the Runs route. A project's (or User's) runs read its
 * workspace roster; under All projects the roster is the recency-ordered
 * aggregate with a Project column and the Project filter. The Steps inventory
 * reads one workspace, so it is offered only with a project or User in scope.
 */
export function useLoopRunsRoute(search: LoopRunsRouteSearch) {
  // The runs listing's profile axis: owner tags under the aggregate, and the
  // profile named when nothing has run yet.
  const profile = useProfileReadScope();
  const { registeredWorkspaces, workspaces } = useActiveWorkspace();
  const { scope, workspaceId } = useLoopWindowScope();
  const allProjects = scope?.kind === "all";
  const projectFilter = useProjectFilter(scope, workspaces);
  const names = new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const enabled = (allProjects || workspaceId !== "") && liveDataEnabled;
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  const navigate = useNavigate({ from: "/loop-runs" });
  const runsQuery = useLoopRunsList(
    loopBreadthTarget(breadthQuery(scope ?? { kind: "all" }, projectFilter.values)),
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

  const inventoryState = allProjects ? undefined : search.nodes;
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
    profile,
    outcome,
    runsQuery,
    setOriginFilter,
    setOutcome,
    allProjects,
    projectFilter,
    /** Under All projects: each run's project, for its Project column. */
    projectOf: allProjects ? (id: string) => workspaceBadgeIdentity(id, names) : undefined,
    /** A data scope is settled: a project, User, or the All projects aggregate. */
    ready: allProjects || workspaceId !== "",
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
