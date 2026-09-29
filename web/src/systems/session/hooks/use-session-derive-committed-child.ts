import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { SessionDeriveCommittedError } from "../adapters/session-derive-api";
import { sessionDetailOptions } from "../lib/query-options";
import {
  openDerivedChild,
  type SessionDerivePlacement,
  type SessionDerivePlacementHandlers,
} from "./use-session-derive";

export interface UseSessionDeriveCommittedChildInput {
  workspaceId: string;
  /** The derive mutation's current error; only a post-commit refusal names a child. */
  error: Error | null;
  placement: SessionDerivePlacement;
  handlers: SessionDerivePlacementHandlers;
  onClose: () => void;
}

/**
 * A continue or fork refused after the new session was already created keeps
 * that session reachable: the dialog offers to open it where the operator chose.
 * The id comes from the daemon's error; the child is read before it opens.
 */
export function useSessionDeriveCommittedChild({
  workspaceId,
  error,
  placement,
  handlers,
  onClose,
}: UseSessionDeriveCommittedChildInput) {
  const queryClient = useQueryClient();
  const [isOpening, setIsOpening] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);
  const childSessionId = error instanceof SessionDeriveCommittedError ? error.childSessionId : null;

  const open = async () => {
    if (!childSessionId || isOpening) return;
    setIsOpening(true);
    setOpenError(null);
    try {
      const child = await queryClient.fetchQuery(sessionDetailOptions(workspaceId, childSessionId));
      onClose();
      openDerivedChild(child, placement, handlers);
    } catch (cause) {
      setOpenError(cause instanceof Error ? cause.message : "Couldn't open the new session.");
    } finally {
      setIsOpening(false);
    }
  };

  return { childSessionId, isOpening, openError, open };
}

export type SessionDeriveCommittedChildModel = ReturnType<typeof useSessionDeriveCommittedChild>;
