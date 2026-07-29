import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { useOsShell } from "@/systems/os";
import { SessionWorkspaceSwitchDialog, type SessionOwnerDialogState } from "@/systems/session";
import { confirmSessionWorkspaceSwitch } from "./-session-workspace-switch-action";

interface SessionWorkspaceSwitchRouteDecisionProps {
  open: boolean;
  owner: SessionOwnerDialogState;
  onReenter: () => void;
  onDecline: () => void;
}

/**
 * Route-private decision orchestration. The foreign session deliberately opens no OS window, so
 * this component owns the coordinator hold for exactly the lifetime of the confirmation route.
 */
export function SessionWorkspaceSwitchRouteDecision({
  open,
  owner,
  onReenter,
  onDecline,
}: SessionWorkspaceSwitchRouteDecisionProps) {
  const { coordinator } = useOsShell();
  const location = useLocation();
  const pathname = location.pathname;
  const search = location.search as Record<string, unknown>;

  useLayoutEffect(() => {
    coordinator.holdRoute({ pathname, search });
    return () => coordinator.releaseRouteHold();
  }, [coordinator, pathname, search]);

  return (
    <SessionWorkspaceSwitchDialog
      open={open}
      workspaceName={owner.workspaceName}
      onConfirm={() => confirmSessionWorkspaceSwitch(owner, onReenter)}
      onCancel={onDecline}
    />
  );
}
