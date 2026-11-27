import { QueryClientContext, type QueryClient } from "@tanstack/react-query";
import { useContext } from "react";

import { useActiveWorkspace, type DataScope } from "@/systems/workspace";

import { profileSessionGroups, projectSessionGroups } from "../lib/session-list-groups";
import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import type { ComputerSessionsSource } from "./use-computer-sessions-sources";
import { scopedCatalogFilters } from "./use-session-catalog";
import { useSessionListLocalState } from "./use-session-list-local-state";
import { useSessionListView, type SessionListViewModel } from "./use-session-list-view";

export function useSessionListComputerView({
  source,
  search,
  scoped,
  worktreeId,
}: {
  source: ComputerSessionsSource;
  search: string;
  scoped: boolean;
  worktreeId?: string;
}) {
  const queryClient = useContext(QueryClientContext);
  const { dataScope } = useActiveWorkspace();
  const offline = source.read === "offline";
  const readOnly = !source.canOperate || offline;

  const computerView = useSessionListView({
    scope: dataScope,
    worktreeId,
    computerId: source.key,
    workspaces: source.workspaces,
    matchedWorkspaceIds: source.matchedWorkspaceIds,
    readOnly,
    search,
    multiComputer: !scoped,
    includePinned: false,
    enabled: !offline,
  });

  const dialogState = useSessionListLocalState(computerView.selectionKey);

  // Offline computers filter rows already cached before disconnect (`Opus 6`, `Astra 15`).
  const listedSessions = computerListRows(queryClient, source, dataScope, computerView);

  const bodyView = offline
    ? offlineComputerListView(computerView, source, dataScope)
    : computerView;
  return { offline, readOnly, computerView, bodyView, listedSessions, dialogState };
}

function computerListRows(
  queryClient: QueryClient | undefined,
  source: ComputerSessionsSource,
  scope: DataScope | null,
  view: SessionListViewModel
) {
  if (source.read !== "offline") return view.catalog?.sessions ?? [];
  if (scope === null) return [];
  const filters = scopedCatalogFilters(
    scope,
    view.groupFilters,
    { all_profiles: true },
    source.key,
    source.matchedWorkspaceIds
  );
  return filters.emptyRemoteMatch
    ? []
    : readCachedOfflineRowsForComputer(queryClient, source.key, filters.scoped);
}

function offlineComputerListView(
  view: SessionListViewModel,
  source: ComputerSessionsSource,
  scope: DataScope | null
): SessionListViewModel {
  if (scope === null || !source.facets) return { ...view, groupBy: "none" };
  const groupInput = { computerId: source.key, offline: true };
  const groups =
    view.groupBy === "project"
      ? projectSessionGroups(source.facets, {
          ...groupInput,
          workspaces: source.workspaces,
          lens: { all_profiles: true },
        })
      : view.groupBy === "profile"
        ? profileSessionGroups(source.facets, {
            ...groupInput,
            scope,
            profiles: source.profiles ?? [],
            matchedWorkspaceIds: source.matchedWorkspaceIds,
          })
        : [];
  return groups.length > 0 ? { ...view, groups } : { ...view, groupBy: "none" };
}
