import { QueryClientContext } from "@tanstack/react-query";
import { useContext, useState } from "react";

import { toast } from "@compozy/ui";

import { isRemoteComputerId, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { computerRegistryKeys, patchComputer } from "@/systems/computers";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import { useComputerSessionsSources } from "./use-computer-sessions-sources";
import { useMultiComputerPinned } from "./use-multi-computer-pinned";
import { useSessionComputerScope } from "./use-session-computer-scope";
import type { SessionListViewModel } from "./use-session-list-view";

export function useSessionListComputers(view: SessionListViewModel) {
  const queryClient = useContext(QueryClientContext);
  const [reconnectingScoped, setReconnectingScoped] = useState(false);
  const { dataScope, registeredWorkspaces } = useActiveWorkspace();
  const profile = useProfileReadScope();

  const computerSources = useComputerSessionsSources({
    scope: dataScope,
    lens: profile.params,
    shelf: view.shelf,
    showAutomation: view.showAutomation,
    search: view.groupFilters.q,
    worktreeId: view.worktreeId,
    includeProfiles: view.groupBy === "profile",
    workspaces: registeredWorkspaces,
  });
  const computerScopeState = useSessionComputerScope();
  const hasRemote = computerSources.hasRemoteComputers;
  const effectiveComputerScope = hasRemote ? computerScopeState.scope : "all";
  const isScopedToLocal = hasRemote && effectiveComputerScope === LOCAL_COMPUTER_ID;
  const scopedRemoteSource = isRemoteComputerId(effectiveComputerScope)
    ? (computerSources.sources.find(source => source.key === effectiveComputerScope) ?? null)
    : null;
  const multiComputerVisible = hasRemote && effectiveComputerScope === "all";
  const computersLayout = view.computers ?? "separate";

  const multiComputerPinned = useMultiComputerPinned({
    enabled: (multiComputerVisible || scopedRemoteSource !== null) && view.shelf === "active",
    sources: scopedRemoteSource ? [scopedRemoteSource] : computerSources.sources,
    scope: dataScope,
    localLens: profile.params,
    groupFilters: view.groupFilters,
    allProjects: view.allProjects === true,
    sort: view.sort,
  });

  const scopedOfflineComputer =
    scopedRemoteSource &&
    (scopedRemoteSource.read === "offline" || scopedRemoteSource.read === "offline_empty") &&
    "lastConnectedLabel" in scopedRemoteSource.computer
      ? scopedRemoteSource.computer
      : null;

  const reconnectScopedComputer = (computerId: string) => {
    setReconnectingScoped(true);
    void patchComputer(computerId, { enabled: true })
      .then(() => queryClient?.invalidateQueries({ queryKey: computerRegistryKeys.all }))
      .catch(err => {
        toast.error(err instanceof Error ? err.message : "Failed to reconnect computer.");
      })
      .finally(() => setReconnectingScoped(false));
  };

  return {
    computerScopeState,
    computerSources,
    computersLayout,
    dataScope,
    isScopedToLocal,
    localLens: profile.params,
    multiComputerPinned,
    multiComputerVisible,
    reconnectingScoped,
    reconnectScopedComputer,
    scopedOfflineComputer,
    scopedRemoteSource,
  };
}
