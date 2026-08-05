import { useState } from "react";
import { toast } from "sonner";

import type { SessionPayload } from "../types";
import {
  useArchiveSession,
  useDeleteSession,
  useStopSession,
  useUnarchiveSession,
} from "./use-session-actions";

export type SessionLifecycleAction = "stop" | "archive" | "unarchive" | "delete";

export interface SessionLifecycleActionHandlers {
  pendingAction: SessionLifecycleAction | null;
  pendingSessionId: string | null;
  onStop: (session: SessionPayload) => void;
  onArchive: (session: SessionPayload) => void;
  onUnarchive: (session: SessionPayload) => void;
  onDelete: (session: SessionPayload) => void;
}

export interface SessionDeleteConfirmation {
  open: boolean;
  session: SessionPayload | null;
  isDeleting: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export interface UseSessionLifecycleActionsResult {
  actions: SessionLifecycleActionHandlers;
  deleteDialog: SessionDeleteConfirmation;
}

export interface UseSessionLifecycleActionsOptions {
  workspaceId?: string | null;
}

function reportActionError(action: SessionLifecycleAction, error: unknown): void {
  const fallback = `Failed to ${action} session.`;
  toast.error(error instanceof Error && error.message ? error.message : fallback);
}

/**
 * Owns the mutation and confirmation state behind session-list lifecycle menus.
 * Lists stay presentational and their host keeps one shared delete confirmation.
 */
export function useSessionLifecycleActions(
  options: UseSessionLifecycleActionsOptions = {}
): UseSessionLifecycleActionsResult {
  const stop = useStopSession(options);
  const archive = useArchiveSession(options);
  const unarchive = useUnarchiveSession(options);
  const remove = useDeleteSession(options);
  const [deleteSession, setDeleteSession] = useState<SessionPayload | null>(null);

  let pendingAction: SessionLifecycleAction | null = null;
  let pendingSessionId: string | null = null;
  if (stop.isPending) {
    pendingAction = "stop";
    pendingSessionId = stop.variables;
  } else if (archive.isPending) {
    pendingAction = "archive";
    pendingSessionId = archive.variables;
  } else if (unarchive.isPending) {
    pendingAction = "unarchive";
    pendingSessionId = unarchive.variables;
  } else if (remove.isPending) {
    pendingAction = "delete";
    pendingSessionId = remove.variables;
  }

  const confirmDelete = () => {
    if (!deleteSession || remove.isPending) return;
    const { id } = deleteSession;
    remove.mutate(id, {
      onError: error => reportActionError("delete", error),
      onSuccess: () => {
        setDeleteSession(current => (current?.id === id ? null : current));
      },
    });
  };

  return {
    actions: {
      pendingAction,
      pendingSessionId,
      onStop: session =>
        stop.mutate(session.id, { onError: error => reportActionError("stop", error) }),
      onArchive: session =>
        archive.mutate(session.id, { onError: error => reportActionError("archive", error) }),
      onUnarchive: session =>
        unarchive.mutate(session.id, { onError: error => reportActionError("unarchive", error) }),
      onDelete: session => {
        if (!remove.isPending) setDeleteSession(session);
      },
    },
    deleteDialog: {
      open: deleteSession !== null,
      session: deleteSession,
      isDeleting: remove.isPending,
      onOpenChange: open => {
        if (!open && !remove.isPending) setDeleteSession(null);
      },
      onConfirm: confirmDelete,
    },
  };
}
