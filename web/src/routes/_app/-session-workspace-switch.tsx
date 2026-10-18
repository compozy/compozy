import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { confirmSessionWorkspaceSwitch } from "./-session-workspace-switch-action";
import { useOsShell } from "@/systems/os";
import { type SessionOwnerDialogState, SessionWorkspaceSwitchDialog } from "@/systems/session";
import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";

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
  // The owner projection identifies Global directly; the retired home row is
  // absent from the project catalog after migration.
  const ownerIsGlobal = owner.workspaceId === "";

  useLayoutEffect(() => {
    coordinator.holdRoute({ pathname, search });
    return () => coordinator.releaseRouteHold();
  }, [coordinator, pathname, search]);

  return (
    <SessionWorkspaceSwitchDialog
      open={open}
      isGlobal={ownerIsGlobal}
      workspaceName={ownerIsGlobal ? GLOBAL_SCOPE_COPY.chipLabel : owner.workspaceName}
      onConfirm={() => {
        confirmSessionWorkspaceSwitch(owner, { isGlobal: ownerIsGlobal }, onReenter);
      }}
      onCancel={onDecline}
    />
  );
}
