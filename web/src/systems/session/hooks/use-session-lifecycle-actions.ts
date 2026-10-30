import { useRef, useState } from "react";
import { toast } from "sonner";

import { runSessionBatch, type SessionBatchResult } from "../lib/session-batch";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { isSubagentSession } from "../lib/session-hierarchy";
import type { SessionPayload } from "../types";
import {
  useArchiveSession,
  useDeleteSession,
  useRenameSession,
  useStopSession,
  useUnarchiveSession,
} from "./use-session-actions";
import { useSessionReparent } from "./use-session-reparent";

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
  /** Nests a top-level session under `parent` (dropped onto its row); absent where unsupported. */
  /** Loaded rows offered by the owning list as candidate parents. */
  nestParents?: readonly SessionPayload[];
  onAdopt?: (session: SessionPayload, parent: SessionPayload) => void;
  /** Returns a nested child to the top level; the row menu's Make top-level. */
  onMakeTopLevel?: (session: SessionPayload) => void;
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

/**
 * Archive always uses the daemon's stop-first operation: a live session stops
 * first, and a stopped parent whose subagent session is still live (until the
 * reaper stops it) archives too. A fully stopped family archives as before.
 */
function archiveTarget(session: SessionPayload) {
  return { ...session, stop: true };
}

/** Preserve the daemon message, falling back only when the failure has no useful text. */
function reportActionError(action: SessionLifecycleAction, error: unknown): void {
  const fallback = `Failed to ${action} session.`;
  toast.error(error instanceof Error && error.message ? error.message : fallback);
}

const sessionCount = (count: number) => `${count} ${count === 1 ? "session" : "sessions"}`;

/**
 * Non-delete batches have no result dialog, so one toast carries the outcome:
 * the success count, and each failed session by name with its own error.
 */
function reportBatch(
  action: "stop" | "archive" | "unarchive",
  results: readonly SessionBatchResult[],
  sessions: readonly SessionPayload[]
): void {
  const verb = action === "stop" ? "stopped" : `${action}d`;
  const done = results.filter(result => result.status === "done").length;
  const failures = results.filter(result => result.status === "failed");
  if (failures.length === 0) {
    toast.success(`${sessionCount(done)} ${verb}`);
    return;
  }
  const byId = new Map(sessions.map(session => [session.id, session]));
  const description = failures
    .map(result => {
      const session = byId.get(result.id);
      const title = session ? getSessionDisplayTitle(session) : result.id;
      return `${title}: ${result.error ?? `Failed to ${action} session.`}`;
    })
    .join("; ");
  const title =
    done > 0
      ? `${sessionCount(done)} ${verb}, ${failures.length} failed`
      : `Couldn't ${action} ${sessionCount(failures.length)}`;
  toast.error(title, { description });
}

/** Report all successful deletes once, when the result dialog closes. */
function reportDeletedSessions(results: readonly SessionBatchResult[]): void {
  const count = results.filter(result => result.status === "done").length;
  if (count > 0) toast.success(`${count} ${count === 1 ? "session" : "sessions"} deleted`);
}

/**
 * Owns the mutation and confirmation state behind session-list lifecycle menus.
 * Lists stay presentational and their host keeps one shared delete confirmation.
 */
export function useSessionLifecycleActions(
  options: Parameters<typeof useDeleteSession>[0] & {
    /** Each session this host archived, single or batched, once the daemon confirmed it. */
    onArchiveSuccess?: (session: SessionPayload) => void;
  } = {}
): UseSessionLifecycleActionsResult {
  const { onArchiveSuccess, ...deleteOptions } = options;
  const stop = useStopSession();
  const archive = useArchiveSession();
  const unarchive = useUnarchiveSession();
  const remove = useDeleteSession(deleteOptions);
  const rename = useRenameSession();
  const reparent = useSessionReparent();
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
    pendingSessionId = archive.variables.id;
  } else if (unarchive.isPending) {
    pendingAction = "unarchive";
    pendingSessionId = unarchive.variables.id;
  } else if (remove.isPending) {
    pendingAction = "delete";
    pendingSessionId = remove.variables.id;
  }

  /** Lock lifecycle controls until the sequential batch has settled every target. */
  const runBatch = async (
    action: "stop" | "archive" | "unarchive" | "delete",
    sessions: readonly SessionPayload[],
    previous: readonly SessionBatchResult[] = []
  ): Promise<SessionBatchResult[] | null> => {
    if (batchRunning.current || pendingAction !== null || sessions.length === 0) return null;
    batchRunning.current = true;
    setBatchAction(action);
    return runSessionBatch({
      action,
      ids: sessions.map(session => session.id),
      previous,
      onProgress: setBatchResults,
      execute: async id => {
        if (action === "delete")
          await remove.mutateAsync(sessions.find(session => session.id === id)!);
        else if (action === "archive") {
          const session = sessions.find(candidate => candidate.id === id)!;
          await archive.mutateAsync(archiveTarget(session));
          onArchiveSuccess?.(session);
        } else if (action === "unarchive")
          await unarchive.mutateAsync(sessions.find(session => session.id === id)!);
        else {
          const outcome = await stop.mutateAsync({
            ...sessions.find(session => session.id === id)!,
            wait: true,
          });
          if (!outcome.verified || outcome.state !== "stopped") {
            throw new Error(outcome.attention || "Session stop could not be verified.");
          }
        }
      },
    }).finally(() => {
      batchRunning.current = false;
      setBatchAction(null);
    });
  };

  /** Apply each verb only to sessions eligible in the current host payload. */
  const actOnMany = async (
    action: "stop" | "archive" | "unarchive",
    sessions: readonly SessionPayload[]
  ) => {
    // Subagent sessions archive with their parent (ADR-005), never on their own.
    // Archive takes live and stopped sessions alike; the daemon stops live ones first.
    const eligible = sessions.filter(session =>
      action === "unarchive"
        ? session.archived_at !== null && !isSubagentSession(session)
        : session.archived_at === null &&
          (action === "archive"
            ? !isSubagentSession(session)
            : session.state === "active" || session.state === "starting")
    );
    const results = await runBatch(action, eligible);
    if (results) reportBatch(action, results, eligible);
  };

  /** Retry failed IDs while preserving successful results across the dialog lifetime. */
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

  /** Keep single-row deletion behavior while routing selection batches through result tracking. */
  const confirmDelete = () => {
    if (!deleteSession || remove.isPending || batchRunning.current) return;
    if (deleteTarget?.bulk) {
      void confirmDeleteMany();
      return;
    }
    const { id } = deleteSession;
    remove.mutate(deleteSession, {
      onError: error => reportActionError("delete", error),
      onSuccess: () => {
        setDeleteTarget(current => (current?.sessions[0]?.id === id ? null : current));
      },
    });
  };

  /** Dismiss only the rename target whose mutation succeeded. */
  const confirmRename = (name: string) => {
    if (!renameTarget || rename.isPending) return;
    const { id } = renameTarget;
    rename.mutate(
      { ...renameTarget, name },
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
        stop.mutate(session, { onError: error => reportActionError("stop", error) }),
      onArchive: session =>
        archive.mutate(archiveTarget(session), {
          onError: error => reportActionError("archive", error),
          onSuccess: () => onArchiveSuccess?.(session),
        }),
      onUnarchive: session =>
        unarchive.mutate(session, { onError: error => reportActionError("unarchive", error) }),
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
      onAdopt: reparent.adopt,
      onMakeTopLevel: reparent.detach,
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
