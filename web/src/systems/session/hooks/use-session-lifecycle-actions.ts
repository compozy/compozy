import {
  batchSkipReason,
  batchSkipNote,
  batchVerbPast,
  reportBatch,
  type BatchVerb,
} from "../lib/session-batch-feedback";
import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { toast } from "sonner";

import {
  readCurrentSessions,
  runSessionBatch,
  type SessionBatchResult,
} from "../lib/session-batch";

import { sessionProcessPorts, sessionsRunningProcesses } from "../lib/session-processes";
import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionPayload } from "../types";
import {
  useArchiveSession,
  useDeleteSession,
  useRenameSession,
  useStopSession,
  useUnarchiveSession,
} from "./use-session-actions";
import { useCurrentDeleteSessions } from "./use-current-delete-sessions";
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
  /** `processes` answers what happens to running processes; absent when none run. */
  onConfirm: (processes?: SessionProcessPolicy) => void;
}

/**
 * The stop/archive question a list asks while its targets have processes
 * running: stop them too, keep them, or cancel. Hosts render it with
 * `SessionProcessChoiceDialog`.
 */
export interface SessionProcessChoiceConfirmation {
  open: boolean;
  action: "stop" | "archive";
  sessionCount: number;
  processCount: number;
  ports: readonly number[];
  onOpenChange: (open: boolean) => void;
  onChoose: (processes: SessionProcessPolicy) => void;
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
  processDialog: SessionProcessChoiceConfirmation;
  renameDialog: SessionRenameConfirmation;
}

/**
 * Archive always uses the daemon's stop-first operation: a live session stops
 * first, and a stopped parent whose subagent session is still live (until the
 * reaper stops it) archives too. A fully stopped family archives as before.
 */
function archiveTarget(session: SessionPayload, processes?: SessionProcessPolicy) {
  return { ...session, stop: true, ...(processes ? { processes } : {}) };
}

/** Preserve the daemon message, falling back only when the failure has no useful text. */
function reportActionError(action: SessionLifecycleAction, error: unknown): void {
  const fallback = `Failed to ${action} session.`;
  toast.error(error instanceof Error && error.message ? error.message : fallback);
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
  const queryClient = useQueryClient();
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
    openedAt: number;
    onSelectionChange?: (ids: readonly string[]) => void;
  } | null>(null);
  const [batchAction, setBatchAction] = useState<SessionLifecycleAction | null>(null);
  const [batchResults, setBatchResults] = useState<readonly SessionBatchResult[]>([]);
  const batchRunning = useRef(false);
  const deleteSessions = useCurrentDeleteSessions(
    deleteTarget?.sessions,
    deleteTarget?.openedAt ?? Infinity
  );
  const deleteSession = deleteSessions?.[0] ?? null;
  const [renameTarget, setRenameTarget] = useState<SessionPayload | null>(null);
  const [processChoice, setProcessChoice] = useState<{
    action: "stop" | "archive";
    sessions: readonly SessionPayload[];
    run: (processes: SessionProcessPolicy) => void;
  } | null>(null);
  // The answer a delete got, so a retry of its failures repeats it.
  const deletePolicy = useRef<SessionProcessPolicy | undefined>(undefined);

  const pendingMutation = (
    [
      ["rename", rename],
      ["stop", stop],
      ["archive", archive],
      ["unarchive", unarchive],
      ["delete", remove],
    ] as const
  ).find(([, mutation]) => mutation.isPending);
  const pendingAction = batchAction ?? pendingMutation?.[0] ?? null;
  const pendingSessionId =
    batchAction !== null
      ? (batchResults.find(result => result.status === "running")?.id ?? null)
      : (pendingMutation?.[1].variables?.id ?? null);

  /** Lock lifecycle controls until the sequential batch has settled every target. */
  const runBatch = async (
    action: "stop" | "archive" | "unarchive" | "delete",
    sessions: readonly SessionPayload[],
    previous: readonly SessionBatchResult[] = [],
    processes?: SessionProcessPolicy,
    readErrors: ReadonlyMap<string, unknown> = new Map()
  ): Promise<SessionBatchResult[] | null> => {
    const policy = processes ? { processes } : {};
    if (batchRunning.current || pendingAction !== null || sessions.length === 0) return null;
    batchRunning.current = true;
    setBatchAction(action);
    return runSessionBatch({
      action,
      ids: sessions.map(session => session.id),
      previous,
      onProgress: setBatchResults,
      execute: async id => {
        if (readErrors.has(id)) throw readErrors.get(id);
        if (action === "delete")
          await remove.mutateAsync({ ...sessions.find(session => session.id === id)!, ...policy });
        else if (action === "archive") {
          const session = sessions.find(candidate => candidate.id === id)!;
          await archive.mutateAsync(archiveTarget(session, processes));
          onArchiveSuccess?.(session);
        } else if (action === "unarchive")
          await unarchive.mutateAsync(sessions.find(session => session.id === id)!);
        else {
          const outcome = await stop.mutateAsync({
            ...sessions.find(session => session.id === id)!,
            wait: true,
            ...policy,
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

  const actOnMany = (action: BatchVerb, sessions: readonly SessionPayload[]) => {
    const eligible: SessionPayload[] = [];
    const skipped = new Map<string, number>();
    for (const session of sessions) {
      const reason = batchSkipReason(action, session);
      if (reason === null) eligible.push(session);
      else skipped.set(reason, (skipped.get(reason) ?? 0) + 1);
    }
    if (eligible.length === 0) {
      const skipNote = batchSkipNote(skipped);
      if (skipNote) toast.info(`No sessions ${batchVerbPast(action)}`, { description: skipNote });
      return;
    }
    const run = (processes?: SessionProcessPolicy, readErrors?: ReadonlyMap<string, unknown>) => {
      void runBatch(action, eligible, [], processes, readErrors).then(results => {
        if (results) reportBatch(action, results, eligible, skipped);
      });
    };
    if (action === "unarchive") run();
    else askAboutProcesses(action, eligible, run);
  };

  /**
   * Stop and archive ask first while their targets have processes running;
   * at zero they run as they always did, with the daemon default (keep).
   */
  const checkingProcesses = useRef(false);
  const askAboutProcesses = (
    action: "stop" | "archive",
    sessions: readonly SessionPayload[],
    run: (processes?: SessionProcessPolicy, readErrors?: ReadonlyMap<string, unknown>) => void
  ) => {
    if (
      sessions.length === 0 ||
      checkingProcesses.current ||
      processChoice !== null ||
      pendingAction !== null ||
      batchRunning.current
    )
      return;
    checkingProcesses.current = true;
    void readCurrentSessions(queryClient, sessions)
      .then(currentSessions => {
        const runChecked = (processes?: SessionProcessPolicy) =>
          run(processes, currentSessions.errors);
        if (sessionsRunningProcesses(currentSessions.sessions) === 0) runChecked();
        else setProcessChoice({ action, sessions: currentSessions.sessions, run: runChecked });
      })
      .catch(error => reportActionError(action, error))
      .finally(() => {
        checkingProcesses.current = false;
      });
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
    const results = await runBatch("delete", targets, batchResults, deletePolicy.current);
    if (!results) return;
    const failures = results.filter(result => result.status === "failed");
    deleteTarget.onSelectionChange?.(failures.map(result => result.id));
    if (failures.length === 0) {
      setDeleteTarget(null);
      reportDeletedSessions(results);
    }
  };

  /** Keep single-row deletion behavior while routing selection batches through result tracking. */
  const performDelete = (processes?: SessionProcessPolicy) => {
    if (!deleteSession || remove.isPending || batchRunning.current) return;
    if (processes) deletePolicy.current = processes;
    if (deleteTarget?.bulk) {
      void confirmDeleteMany();
      return;
    }
    const { id } = deleteSession;
    const policy = deletePolicy.current ? { processes: deletePolicy.current } : {};
    remove.mutate(
      { ...deleteSession, ...policy },
      {
        onError: error => reportActionError("delete", error),
        onSuccess: () => {
          setDeleteTarget(current => (current?.sessions[0]?.id === id ? null : current));
        },
      }
    );
  };

  const confirmDelete = (processes?: SessionProcessPolicy) => {
    if (!deleteTarget) return;
    void readCurrentSessions(queryClient, deleteTarget.sessions)
      .then(current => {
        if (current.errors.size) throw current.errors.values().next().value;
        // A fresh nonzero count must be answered in the now-updated dialog.
        if (!processes && !deletePolicy.current && sessionsRunningProcesses(current.sessions) > 0)
          return;
        performDelete(processes);
      })
      .catch(error => reportActionError("delete", error));
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
        void askAboutProcesses("stop", [session], processes =>
          stop.mutate(
            { ...session, ...(processes ? { processes } : {}) },
            { onError: error => reportActionError("stop", error) }
          )
        ),
      onArchive: session =>
        void askAboutProcesses("archive", [session], processes =>
          archive.mutate(archiveTarget(session, processes), {
            onError: error => reportActionError("archive", error),
            onSuccess: () => onArchiveSuccess?.(session),
          })
        ),
      onUnarchive: session =>
        unarchive.mutate(session, { onError: error => reportActionError("unarchive", error) }),
      onDelete: session => {
        if (pendingAction === null && !batchRunning.current) {
          deletePolicy.current = undefined;
          setBatchResults([]);
          setDeleteTarget({ sessions: [session], bulk: false, openedAt: Date.now() });
        }
      },
      onStopMany: sessions => actOnMany("stop", sessions),
      onArchiveMany: sessions => actOnMany("archive", sessions),
      onUnarchiveMany: sessions => actOnMany("unarchive", sessions),
      onAdopt: reparent.adopt,
      onMakeTopLevel: reparent.detach,
      onDeleteMany: (sessions, onSelectionChange) => {
        if (pendingAction === null && !batchRunning.current && sessions.length > 0) {
          deletePolicy.current = undefined;
          setBatchResults([]);
          setDeleteTarget({
            sessions: [...sessions],
            bulk: true,
            openedAt: Date.now(),
            onSelectionChange,
          });
        }
      },
    },
    deleteDialog: {
      open: deleteSession !== null,
      session: deleteSession,
      sessions: deleteSessions,
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
      onRetry: () => confirmDelete(),
    },
    processDialog: {
      open: processChoice !== null,
      action: processChoice?.action ?? "stop",
      sessionCount: processChoice?.sessions.length ?? 0,
      processCount: sessionsRunningProcesses(processChoice?.sessions ?? []),
      ports: (processChoice?.sessions ?? []).flatMap(session => sessionProcessPorts(session)),
      onOpenChange: open => {
        if (!open) setProcessChoice(null);
      },
      onChoose: processes => {
        const choice = processChoice;
        setProcessChoice(null);
        choice?.run(processes);
      },
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
