import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useEffectEvent } from "react";

import { approveSession, type PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";
import { createSessionPermissionDecisionLogic } from "./session-permission-decision-store";

const sessionPermissionDecisionLogic = createSessionPermissionDecisionLogic();

interface UseSessionPermissionDecisionOptions {
  workspaceId: string;
  sessionId: string;
  permission: PermissionRequest;
  onResolved?: () => void;
}

export function useSessionPermissionDecision({
  workspaceId,
  sessionId,
  permission,
  onResolved,
}: UseSessionPermissionDecisionOptions) {
  const store = useStore(sessionPermissionDecisionLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const permissionKey = `${permission.requestId}\u0000${permission.turnId ?? ""}`;
  const isCurrentPermission = state.permissionKey === permissionKey;
  const handleResolved = useEffectEvent(() => onResolved?.());

  useEffect(() => {
    const resolved = store.on("resolutionAccepted", handleResolved);
    return () => {
      resolved.unsubscribe();
    };
  }, [store]);

  const decide = (decision: PermissionDecision) =>
    store.trigger.decisionSubmitted({
      decision,
      permission,
      permissionKey,
      sessionId,
      workspaceId,
      execute: input =>
        approveSession(input.workspaceId, input.sessionId, {
          request_id: input.permission.requestId,
          turn_id: input.permission.turnId ?? "",
          decision: input.decision,
        }),
    });

  return {
    decide,
    isResolved: isCurrentPermission && state.phase === "resolved",
    isSubmitting: isCurrentPermission && state.phase === "submitting",
  };
}
