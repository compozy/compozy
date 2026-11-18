import { useSelector, useStore } from "@xstate/store-react";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useEffectEvent } from "react";

import { approveSession, type PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";
import { sessionExpiredInteractionsOptions } from "../lib/query-options";
import { permissionOccurrenceKey } from "../lib/session-pending-interactions";
import { createSessionPermissionDecisionLogic } from "./session-permission-decision-store";

const sessionPermissionDecisionLogic = createSessionPermissionDecisionLogic();

interface UseSessionPermissionDecisionOptions {
  workspaceId: string;
  sessionId: string;
  permission: PermissionRequest;
  /** Runs once the daemon has accepted the decision, never before. */
  onResolved?: (decision: PermissionDecision) => void;
}

export function useSessionPermissionDecision({
  workspaceId,
  sessionId,
  permission,
  onResolved,
}: UseSessionPermissionDecisionOptions) {
  const store = useStore(sessionPermissionDecisionLogic);
  const queryClient = useQueryClient();
  const state = useSelector(store, snapshot => snapshot.context);
  const permissionKey = permissionOccurrenceKey(permission);
  const isCurrentPermission = state.permissionKey === permissionKey;
  const handleResolved = useEffectEvent(({ decision }: { decision: PermissionDecision }) =>
    onResolved?.(decision)
  );

  useEffect(() => {
    const resolved = store.on("resolutionAccepted", handleResolved);
    const unavailable = store.on("permissionUnavailable", () => {
      void queryClient.invalidateQueries({
        queryKey: sessionExpiredInteractionsOptions(workspaceId, sessionId).queryKey,
      });
    });
    return () => {
      resolved.unsubscribe();
      unavailable.unsubscribe();
    };
  }, [store, queryClient, workspaceId, sessionId]);

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
    isUnavailable: isCurrentPermission && state.phase === "unavailable",
    isSubmitting: isCurrentPermission && state.phase === "submitting",
  };
}
