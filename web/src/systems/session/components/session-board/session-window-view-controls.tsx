import { ComputerScopePicker, useComputers, type ComputerScopeCount } from "@/systems/computers";

import { useSessionWindowComputerControls } from "../../hooks/use-session-window-computer-controls";
import type { SessionWindowViewMode } from "../../lib/session-sidebar-scope";
import { SessionViewToggle } from "./session-view-toggle";

export interface SessionWindowViewControlsProps {
  value: SessionWindowViewMode;
  onValueChange: (next: SessionWindowViewMode) => void;
  /** Optional handler to open Settings → Computers from the scope picker footer. */
  onManageComputers?: () => void;
}

/**
 * Shared Sessions window topbar controls: List | Kanban toggle plus the per-window
 * computer scope picker (`F8`). Mounted at all three Sessions window sites so they
 * stay identical; the picker hides when no remote computer is enabled (`N6`) without
 * firing workspace or profile reads.
 */
export function SessionWindowViewControls({
  value,
  onValueChange,
  onManageComputers,
}: SessionWindowViewControlsProps) {
  const { canRead, enabledComputers } = useComputers({ passive: true });
  if (!canRead || enabledComputers.length === 0) {
    return <SessionViewToggle onValueChange={onValueChange} value={value} />;
  }
  return (
    <SessionWindowMultiComputerControls
      onManageComputers={onManageComputers}
      onValueChange={onValueChange}
      value={value}
    />
  );
}

function SessionWindowMultiComputerControls({
  value,
  onValueChange,
  onManageComputers,
}: SessionWindowViewControlsProps) {
  const { computerSources, handleReconnect, reconnectingIds, scopeState, upgradeSheet } =
    useSessionWindowComputerControls();

  if (!computerSources.hasRemoteComputers) {
    return <SessionViewToggle onValueChange={onValueChange} value={value} />;
  }

  const countsByComputer = new Map<string, ComputerScopeCount>(
    computerSources.sources.map(source => [
      source.key,
      { count: source.count, needsYou: source.needsYou },
    ])
  );

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
        value={scopeState.scope}
      />
    </span>
  );
}
