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
  placement: SessionDerivePlacement;
  handlers: SessionDerivePlacementHandlers;
  onClose: () => void;
}

/**
 * A continue or fork refused after the new session was already created keeps
 * that session reachable: the dialog offers to open it where the operator chose.
 * The id comes from the daemon's error and is kept for the rest of the dialog:
 * a later refusal (an edited request reusing the key is an idempotency
 * conflict that names no child) or a form edit never hides it. The child is
 * read before it opens.
 */
export function useSessionDeriveCommittedChild({
  workspaceId,
  placement,
  handlers,
  onClose,
}: UseSessionDeriveCommittedChildInput) {
  const queryClient = useQueryClient();
  const [childSessionId, setChildSessionId] = useState<string | null>(null);
  const [isOpening, setIsOpening] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);

  /** Records the child a failed submit names; the first committed child stays. */
  const noteFailure = (error: Error) => {
    if (error instanceof SessionDeriveCommittedError) {
      setChildSessionId(current => current ?? error.childSessionId);
    }
  };

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

  return { childSessionId, isOpening, openError, open, noteFailure };
}

export type SessionDeriveCommittedChildModel = ReturnType<typeof useSessionDeriveCommittedChild>;
