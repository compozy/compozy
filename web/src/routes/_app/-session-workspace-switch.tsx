import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { confirmSessionWorkspaceSwitch } from "./-session-workspace-switch-action";
import { useOsShell } from "@/systems/os";
import { type SessionOwnerDialogState, SessionWorkspaceSwitchDialog } from "@/systems/session";
import { GLOBAL_SCOPE_COPY, useActiveWorkspace } from "@/systems/workspace";

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
  const { homeWorkspace } = useActiveWorkspace();
  const location = useLocation();
  const pathname = location.pathname;
  const search = location.search as Record<string, unknown>;
  // A session owned by the operator-home row is a Global session, not a
  // foreign workspace — confirming turns Global scope on instead of selecting it.
  const ownerIsGlobal = owner.workspaceId === homeWorkspace?.id;

  useLayoutEffect(() => {
    coordinator.holdRoute({ pathname, search });
    return () => coordinator.releaseRouteHold();
  }, [coordinator, pathname, search]);

  return (
    <SessionWorkspaceSwitchDialog
      open={open}
      isGlobal={ownerIsGlobal}
      workspaceName={ownerIsGlobal ? GLOBAL_SCOPE_COPY.chipLabel : owner.workspaceName}
      onConfirm={() => confirmSessionWorkspaceSwitch(owner, { isGlobal: ownerIsGlobal }, onReenter)}
      onCancel={onDecline}
    />
  );
}
