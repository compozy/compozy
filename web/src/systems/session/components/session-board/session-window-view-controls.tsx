import { QueryClientContext } from "@tanstack/react-query";
import { useContext, useState } from "react";

import { toast } from "@compozy/ui";

import { useOptionalUpgradeSheetActions } from "@/systems/account";
import {
  ComputerScopePicker,
  computerRegistryKeys,
  patchComputer,
  type ComputerScopeCount,
} from "@/systems/computers";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import { useComputerSessionsSources } from "../../hooks/use-computer-sessions-sources";
import { useSessionComputerScope } from "../../hooks/use-session-computer-scope";
import { useSessionSidebarState } from "../../hooks/use-session-sidebar-state";
import type { SessionWindowView } from "../../lib/session-board-view";
import { SessionViewToggle } from "./session-view-toggle";

export interface SessionWindowViewControlsProps {
  value: SessionWindowView;
  onValueChange: (next: SessionWindowView) => void;
  /** Optional handler to open Settings → Computers from the scope picker footer. */
  onManageComputers?: () => void;
}

/**
 * Shared Sessions window topbar controls: List | Kanban toggle plus the per-window
 * computer scope picker (`F8`). Mounted at all three Sessions window sites so they
 * stay identical; the picker hides when no remote computer is enabled (`N6`).
 */
export function SessionWindowViewControls({
  value,
  onValueChange,
  onManageComputers,
}: SessionWindowViewControlsProps) {
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
  const scopeState = useSessionComputerScope(computerSources.remoteComputers);

  if (!computerSources.hasRemoteComputers) {
    return <SessionViewToggle onValueChange={onValueChange} value={value} />;
  }

  const countsByComputer = new Map<string, ComputerScopeCount>(
    computerSources.sources.map(source => [
      source.key,
      { count: source.count, needsYou: source.needsYou },
    ])
  );

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

  return (
    <span className="flex items-center gap-2">
      <SessionViewToggle onValueChange={onValueChange} value={value} />
      <ComputerScopePicker
        computers={computerSources.remoteComputers}
        countsByComputer={countsByComputer}
        local={computerSources.local}
        onChange={scopeState.setScope}
        onManageComputers={onManageComputers}
        onReconnect={handleReconnect}
        onUpgrade={upgradeSheet ? () => upgradeSheet.openUpgradeSheet() : undefined}
        reconnectingIds={reconnectingIds}
        value={scopeState.effectiveScope}
      />
    </span>
  );
}
