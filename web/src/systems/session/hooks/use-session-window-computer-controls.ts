import { QueryClientContext } from "@tanstack/react-query";
import { useContext, useState } from "react";

import { toast } from "@compozy/ui";

import { useOptionalUpgradeSheetActions } from "@/systems/account";
import { computerRegistryKeys, patchComputer } from "@/systems/computers";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import { useComputerSessionsSources } from "./use-computer-sessions-sources";
import { useSessionComputerScope } from "./use-session-computer-scope";
import { useSessionSidebarState } from "./use-session-sidebar-state";

export function useSessionWindowComputerControls() {
  const queryClient = useContext(QueryClientContext);
  const upgradeSheet = useOptionalUpgradeSheetActions();
  const { dataScope } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const sidebar = useSessionSidebarState();
  const [reconnectingIds, setReconnectingIds] = useState<string[]>([]);

  const computerSources = useComputerSessionsSources({
    scope: dataScope,
    lens: profile.params,
    shelf: sidebar.shelf,
    showAutomation: sidebar.showAutomation,
  });
  const scopeState = useSessionComputerScope();

  const handleReconnect = (computerId: string) => {
    setReconnectingIds(prev => (prev.includes(computerId) ? prev : [...prev, computerId]));
    void patchComputer(computerId, { enabled: true })
      .then(() => queryClient?.invalidateQueries({ queryKey: computerRegistryKeys.all }))
      .catch(err => {
        toast.error(err instanceof Error ? err.message : "Failed to reconnect computer.");
      })
      .finally(() => {
        setReconnectingIds(prev => prev.filter(id => id !== computerId));
      });
  };

  return { computerSources, handleReconnect, reconnectingIds, scopeState, upgradeSheet };
}
