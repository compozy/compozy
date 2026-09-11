import { useRef, useState } from "react";
import { toast } from "sonner";

import type { SessionBatchResult } from "../lib/session-batch";

import type { SessionPayload } from "../types";
import {
  useArchiveSession,
  useDeleteSession,
  useRenameSession,
  useStopSession,
  useUnarchiveSession,
} from "./use-session-actions";

export type SessionLifecycleAction = "rename" | "stop" | "archive" | "unarchive" | "delete";

export interface SessionLifecycleActionHandlers {
  pendingAction: SessionLifecycleAction | null;
  pendingSessionId: string | null;
  onStop: (session: SessionPayload) => void;
  onRename: (session: SessionPayload) => void;
  onArchive: (session: SessionPayload) => void;
  onUnarchive: (session: SessionPayload) => void;
  onDelete: (session: SessionPayload) => void;
  onStopMany?: (sessions: readonly SessionPayload[]) => void;
  onArchiveMany?: (sessions: readonly SessionPayload[]) => void;
  onUnarchiveMany?: (sessions: readonly SessionPayload[]) => void;
  onDeleteMany?: (
    sessions: readonly SessionPayload[],
    onSelectionChange?: (remainingIds: readonly string[]) => void
  ) => void;
}

export interface SessionDeleteConfirmation {
  sessions?: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  onRetry?: () => void;
  open: boolean;
  session: SessionPayload | null;
  isDeleting: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export interface SessionRenameConfirmation {
  open: boolean;
  session: SessionPayload | null;
  isRenaming: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (name: string) => void;
}

export interface UseSessionLifecycleActionsResult {
  actions: SessionLifecycleActionHandlers;
  batchResults: readonly SessionBatchResult[];
  deleteDialog: SessionDeleteConfirmation;
  renameDialog: SessionRenameConfirmation;
}

export interface UseSessionLifecycleActionsOptions {
  workspaceId?: string | null;
}

function reportActionError(action: SessionLifecycleAction, error: unknown): void {
  const fallback = `Failed to ${action} session.`;
  toast.error(error instanceof Error && error.message ? error.message : fallback);
}

function reportBatch(
  action: "stop" | "archive" | "unarchive",
  results: readonly SessionBatchResult[]
): void {
  const failures = results.filter(result => result.status === "failed");
  if (failures.length > 0) {
    reportActionError(action, new Error(failures.map(result => result.error).join("; ")));
    return;
  }
  const count = results.filter(result => result.status === "done").length;
  const verb = action === "stop" ? "stopped" : `${action}d`;
  toast.success(`${count} ${count === 1 ? "session" : "sessions"} ${verb}`);
}

function reportDeletedSessions(results: readonly SessionBatchResult[]): void {
  const count = results.filter(result => result.status === "done").length;
  if (count > 0) toast.success(`${count} ${count === 1 ? "session" : "sessions"} deleted`);
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
  const rename = useRenameSession(options);
  const [deleteTarget, setDeleteTarget] = useState<{
    sessions: readonly SessionPayload[];
    bulk: boolean;
    onSelectionChange?: (ids: readonly string[]) => void;
  } | null>(null);
  const [batchAction, setBatchAction] = useState<SessionLifecycleAction | null>(null);
  const [batchResults, setBatchResults] = useState<readonly SessionBatchResult[]>([]);
  const batchRunning = useRef(false);
  const deleteSession = deleteTarget?.sessions[0] ?? null;
  const [renameTarget, setRenameTarget] = useState<SessionPayload | null>(null);

  let pendingAction: SessionLifecycleAction | null = null;
  let pendingSessionId: string | null = null;
  if (batchAction !== null) {
    pendingAction = batchAction;
    pendingSessionId = batchResults.find(result => result.status === "running")?.id ?? null;
  } else if (rename.isPending) {
    pendingAction = "rename";
    pendingSessionId = rename.variables.id;
  } else if (stop.isPending) {
    pendingAction = "stop";
    pendingSessionId = stop.variables.id;
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

  const runBatch = async (
    action: "stop" | "archive" | "unarchive" | "delete",
    sessions: readonly SessionPayload[],
    previous: readonly SessionBatchResult[] = []
  ): Promise<SessionBatchResult[] | null> => {
    if (batchRunning.current || pendingAction !== null || sessions.length === 0) return null;
    batchRunning.current = true;
    setBatchAction(action);
    const targets = new Set(sessions.map(session => session.id));
    const results: SessionBatchResult[] =
      previous.length > 0
        ? previous.map(result =>
            targets.has(result.id) ? { id: result.id, status: "pending" } : result
          )
        : sessions.map(session => ({ id: session.id, status: "pending" }));
    const update = (result: SessionBatchResult) => {
      const index = results.findIndex(current => current.id === result.id);
      results[index] = result;
      setBatchResults([...results]);
    };
    setBatchResults([...results]);
    try {
      for (const session of sessions) {
        update({ id: session.id, status: "running" });
        try {
          if (action === "delete") await remove.mutateAsync(session.id);
          else if (action === "archive") await archive.mutateAsync(session.id);
          else if (action === "unarchive") await unarchive.mutateAsync(session.id);
          else {
            const outcome = await stop.mutateAsync({ id: session.id, wait: true });
            if (!outcome.verified || outcome.state !== "stopped") {
              throw new Error(outcome.attention || "Session stop could not be verified.");
            }
          }
          update({ id: session.id, status: "done" });
        } catch (error) {
          update({
            id: session.id,
            status: "failed",
            error:
              error instanceof Error && error.message
                ? error.message
                : `Failed to ${action} session.`,
          });
        }
      }
      return results;
    } finally {
      batchRunning.current = false;
      setBatchAction(null);
    }
  };

  const actOnMany = async (
    action: "stop" | "archive" | "unarchive",
    sessions: readonly SessionPayload[]
  ) => {
    const eligible = sessions.filter(session =>
      action === "unarchive"
        ? session.archived_at !== null
        : session.archived_at === null &&
          (action === "archive"
            ? session.state === "stopped"
            : session.state === "active" || session.state === "starting")
    );
    const results = await runBatch(action, eligible);
    if (results) reportBatch(action, results);
  };

  const confirmDeleteMany = async () => {
    if (!deleteTarget) return;
    const failedIds = new Set(
      batchResults.flatMap(result => (result.status === "failed" ? [result.id] : []))
    );
    const targets =
      failedIds.size > 0
        ? deleteTarget.sessions.filter(session => failedIds.has(session.id))
        : deleteTarget.sessions;
    const results = await runBatch("delete", targets, batchResults);
    if (!results) return;
    const failures = results.filter(result => result.status === "failed");
    deleteTarget.onSelectionChange?.(failures.map(result => result.id));
    if (failures.length === 0) {
      setDeleteTarget(null);
      reportDeletedSessions(results);
    }
  };

  const confirmDelete = () => {
    if (!deleteSession || remove.isPending || batchRunning.current) return;
    if (deleteTarget?.bulk) {
      void confirmDeleteMany();
      return;
    }
    const { id } = deleteSession;
    remove.mutate(id, {
      onError: error => reportActionError("delete", error),
      onSuccess: () => {
        setDeleteTarget(current => (current?.sessions[0]?.id === id ? null : current));
      },
    });
  };

  const confirmRename = (name: string) => {
    if (!renameTarget || rename.isPending) return;
    const { id } = renameTarget;
    rename.mutate(
      { id, name },
      {
        onError: error => reportActionError("rename", error),
        onSuccess: () => {
          setRenameTarget(current => (current?.id === id ? null : current));
        },
      }
    );
  };

  return {
    batchResults,
    actions: {
      pendingAction,
      pendingSessionId,
      onRename: session => {
        if (pendingAction === null && !batchRunning.current) setRenameTarget(session);
      },
      onStop: session =>
        stop.mutate({ id: session.id }, { onError: error => reportActionError("stop", error) }),
      onArchive: session =>
        archive.mutate(session.id, { onError: error => reportActionError("archive", error) }),
      onUnarchive: session =>
        unarchive.mutate(session.id, { onError: error => reportActionError("unarchive", error) }),
      onDelete: session => {
        if (pendingAction === null && !batchRunning.current) {
          setBatchResults([]);
          setDeleteTarget({ sessions: [session], bulk: false });
        }
      },
      onStopMany: sessions => {
        void actOnMany("stop", sessions);
      },
      onArchiveMany: sessions => {
        void actOnMany("archive", sessions);
      },
      onUnarchiveMany: sessions => {
        void actOnMany("unarchive", sessions);
      },
      onDeleteMany: (sessions, onSelectionChange) => {
        if (pendingAction === null && !batchRunning.current && sessions.length > 0) {
          setBatchResults([]);
          setDeleteTarget({ sessions: [...sessions], bulk: true, onSelectionChange });
        }
      },
    },
    deleteDialog: {
      open: deleteSession !== null,
      session: deleteSession,
      sessions: deleteTarget?.sessions,
      results: deleteTarget?.bulk ? batchResults : undefined,
      isDeleting: remove.isPending || batchAction === "delete",
      onOpenChange: open => {
        if (!open && deleteTarget && !remove.isPending && !batchRunning.current) {
          if (batchResults.some(result => result.status === "failed")) {
            deleteTarget?.onSelectionChange?.(
              batchResults.flatMap(result => (result.status === "failed" ? [result.id] : []))
            );
          }
          if (deleteTarget.bulk) reportDeletedSessions(batchResults);
          setDeleteTarget(null);
        }
      },
      onConfirm: confirmDelete,
      onRetry: confirmDelete,
    },
    renameDialog: {
      open: renameTarget !== null,
      session: renameTarget,
      isRenaming: rename.isPending,
      onOpenChange: open => {
        if (!open && !rename.isPending) setRenameTarget(null);
      },
      onConfirm: confirmRename,
    },
  };
}
