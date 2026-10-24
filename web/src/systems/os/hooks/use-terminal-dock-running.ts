"use client";

import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import {
  terminalAggregateQuery,
  terminalAggregateScope,
  terminalCatalogQuery,
  terminalScope,
  terminalsRunning,
  useTerminalAggregateStream,
} from "@/systems/terminal";
import { focusedWorkspaceSelector, useDataScope } from "@/systems/workspace";

/**
 * Whether any terminal the Terminal listing would show is alive: the focused
 * project's or User's, or — under All projects — every project's, read from
 * the aggregate kept live by its stream. Always within the profile lens.
 *
 * The dock's live mark is catalog truth, not "a Terminal window is open".
 * An empty query or a scope that is not ready is idle — never a guess from
 * window state.
 */
export function useTerminalDockRunning(): boolean {
  const dataScope = useDataScope();
  const profile = useProfileReadScope();
  const allProjects = dataScope?.kind === "all";
  const workspaceId = focusedWorkspaceSelector(dataScope);
  const catalog = useQuery({
    ...terminalCatalogQuery(terminalScope(workspaceId, profile.destination, profile.aggregate)),
    enabled: workspaceId !== "",
  });
  const aggregateScope = terminalAggregateScope([], profile.destination, profile.aggregate);
  const aggregate = useQuery({ ...terminalAggregateQuery(aggregateScope), enabled: allProjects });
  useTerminalAggregateStream({ scope: aggregateScope, enabled: allProjects });
  const terminals = allProjects ? aggregate.data : workspaceId === "" ? undefined : catalog.data;
  return terminals !== undefined && terminalsRunning(terminals);
}
