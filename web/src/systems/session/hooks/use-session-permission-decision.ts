import { useRef, useState } from "react";
import { toast } from "sonner";

import { approveSession, type PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";

interface UseSessionPermissionDecisionOptions {
  workspaceId: string;
  sessionId: string;
  permission: PermissionRequest;
  onResolved?: () => void;
}

async function submitPermissionDecision(
  workspaceId: string,
  sessionId: string,
  permission: PermissionRequest,
  decision: PermissionDecision,
  onApproved: () => void,
  onSettled: () => void
): Promise<void> {
  try {
    await approveSession(workspaceId, sessionId, {
      request_id: permission.requestId,
      turn_id: permission.turnId ?? "",
      decision,
    });
    onApproved();
  } catch (error) {
    console.error("Failed to submit permission decision", error);
    toast.error("Failed to send permission response. The agent may continue waiting.");
  } finally {
    onSettled();
  }
}

export function useSessionPermissionDecision({
  workspaceId,
  sessionId,
  permission,
  onResolved,
}: UseSessionPermissionDecisionOptions) {
  const submittingRef = useRef(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isResolved, setIsResolved] = useState(false);
  const decide = (decision: PermissionDecision): Promise<void> => {
    if (submittingRef.current) return Promise.resolve();
    submittingRef.current = true;
    setIsSubmitting(true);
    return submitPermissionDecision(
      workspaceId,
      sessionId,
      permission,
      decision,
      () => {
        setIsResolved(true);
        onResolved?.();
      },
      () => {
        submittingRef.current = false;
        setIsSubmitting(false);
      }
    );
  };

  return {
    decide,
    isResolved,
    isSubmitting,
  };
}
