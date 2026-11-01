import {
  batchSkipReason,
  batchSkipNote,
  batchVerbPast,
  reportBatch,
  type BatchVerb,
} from "../lib/session-batch-feedback";
import { useQueryClient } from "@tanstack/react-query";

import { useRef, useState } from "react";
import { toast } from "@compozy/ui";

import {
  lifecyclePendingSessionId,
  readCurrentSessions,
  runSessionBatch,
  type SessionBatchResult,
} from "../lib/session-batch";

import { sessionProcessPorts, sessionsRunningProcesses } from "../lib/session-processes";
import {
  readArchivedDescendants,
  readSessionChoiceTargets,
  sessionBatchCoverage,
  sessionChildrenChoice,
  sessionChildrenScope,
  sessionTreeBatchOrder,
  type SessionChildrenScope,
  type SessionTrees,
} from "../lib/session-tree";
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
import { useSessionDeleteTrees } from "./use-session-delete-trees";
import {
  archiveTarget,
  descendantsOf,
  sessionArchiveChoiceState,
  reportActionError,
  reportDeletedSessions,
  settleTreeTarget,
} from "./session-lifecycle-helpers";
import { useSessionReparent } from "./use-session-reparent";
import { useSessionLaterActions } from "./use-session-later-actions";
import type {
  SessionLifecycleAction,
  UseSessionLifecycleActionsResult,
} from "./session-lifecycle-types";

export type {
  SessionDeleteConfirmation,
  SessionLifecycleAction,
  SessionLifecycleActionHandlers,
  SessionProcessChoiceConfirmation,
  SessionRenameConfirmation,
  UseSessionLifecycleActionsResult,
} from "./session-lifecycle-types";

/**
 * Owns the mutation and confirmation state behind session-list lifecycle menus.
 * Lists stay presentational and their host keeps one shared delete confirmation.
 */
export function useSessionLifecycleActions(
  options: Parameters<typeof useDeleteSession>[0] & {
    /** Each session this host archived, single or batched, once the daemon confirmed it. */
    onArchiveSuccess?: (session: SessionPayload) => void | Promise<void>;
    /** Each session this host moved to Later, single or batched, once the daemon confirmed it. */
    onLaterSuccess?: (session: SessionPayload) => void;
  } = {}
): UseSessionLifecycleActionsResult {
  const queryClient = useQueryClient();
  const { onArchiveSuccess, onLaterSuccess, ...deleteOptions } = options;
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
  const deleteConfirmation = useRef(0);
  const [deleteChildren, setDeleteChildren] = useState(false);
  const deleteSessions = useCurrentDeleteSessions(
    deleteTarget?.sessions,
    deleteTarget?.openedAt ?? Infinity
  );
  const deleteSession = deleteSessions?.[0] ?? null;
  // A target the batch already deleted has no tree left to read or to ask about.
  const remainingDeleteTargets = deleteTarget?.sessions.filter(
    session => !batchResults.some(result => result.id === session.id && result.status === "done")
  );
  const deleteRead = useSessionDeleteTrees(remainingDeleteTargets);
  const deleteTrees = deleteRead.trees;
  const deleteScope = sessionChildrenScope("delete", remainingDeleteTargets ?? [], deleteTrees);
  // Only what the dialog shows is sent: with no choice on screen, a delete stays single-session.
  const includedDeleteTrees =
    deleteChildren && deleteScope.members.length > 0 ? deleteTrees : undefined;
  const [renameTarget, setRenameTarget] = useState<SessionPayload | null>(null);
  const [processChoice, setProcessChoice] = useState<{
    action: "stop" | "archive";
    sessions: readonly SessionPayload[];
    scope?: SessionChildrenScope;
    run: (processes?: SessionProcessPolicy, withChildren?: boolean) => void;
  } | null>(null);
  const [archiveChildren, setArchiveChildren] = useState(true);
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
  const ownPending = batchAction ?? pendingMutation?.[0] ?? null;
  const later = useSessionLaterActions({
    busy: ownPending !== null,
    onMovedToLater: onLaterSuccess,
  });
  const pendingAction = ownPending ?? later.pendingAction;
  const pendingSessionId = lifecyclePendingSessionId({
    batching: batchAction !== null,
    batchResults,
    ownPending: ownPending !== null,
    mutationSessionId: pendingMutation?.[1].variables?.id,
    laterSessionId: later.pendingSessionId,
  });

  /** A host's follow-up failing does not undo an archive the daemon confirmed. */
  const notifyArchived = async (session: SessionPayload, trees?: SessionTrees) => {
    const children = await readArchivedDescendants(queryClient, trees?.get(session.id) ?? []);
    const results = await Promise.allSettled(
      [session, ...children].map(target => onArchiveSuccess?.(target))
    );
    for (const result of results) {
      if (result.status === "rejected") reportActionError("archive", result.reason);
    }
  };

  /** Lock lifecycle controls until the sequential batch has settled every target. */
  const runBatch = async (
    action: "stop" | "archive" | "unarchive" | "delete",
    sessions: readonly SessionPayload[],
    previous: readonly SessionBatchResult[] = [],
    processes?: SessionProcessPolicy,
    readErrors: ReadonlyMap<string, unknown> = new Map(),
    trees?: SessionTrees
  ): Promise<SessionBatchResult[] | null> => {
    const policy = processes ? { processes } : {};
    if (batchRunning.current || pendingAction !== null || sessions.length === 0) return null;
    batchRunning.current = true;
    setBatchAction(action);
    const coveredBy = sessionBatchCoverage(action, sessions, trees);
    return runSessionBatch({
      action,
      ids: sessionTreeBatchOrder(sessions, coveredBy).map(session => session.id),
      previous,
      onProgress: setBatchResults,
      execute: async id => {
        const session = sessions.find(candidate => candidate.id === id)!;
        if (action === "delete" || action === "archive") {
          await settleTreeTarget(action, coveredBy.has(id), async () => {
            if (readErrors.has(id)) throw readErrors.get(id);
            if (action === "delete")
              await remove.mutateAsync({ ...session, ...policy, ...descendantsOf(session, trees) });
            else {
              await archive.mutateAsync(archiveTarget(session, processes, trees));
              await notifyArchived(session, trees);
            }
          });
          return;
        }
        if (readErrors.has(id)) throw readErrors.get(id);
        if (action === "unarchive")
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

  const actOnMany = (
    action: Exclude<BatchVerb, "later" | "back-from-later">,
    sessions: readonly SessionPayload[]
  ) => {
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
    const run = (
      processes?: SessionProcessPolicy,
      readErrors?: ReadonlyMap<string, unknown>,
      trees?: SessionTrees
    ) => {
      void runBatch(action, eligible, [], processes, readErrors, trees).then(results => {
        if (results) reportBatch(action, results, eligible, skipped);
      });
    };
    if (action === "unarchive") run();
    else askAboutProcesses(action, eligible, run);
  };

  /**
   * Stop and archive ask first while their targets have processes running;
   * at zero they run as they always did, with the daemon default (keep).
   * Archive also asks while its targets have child sessions to include.
   */
  const checkingProcesses = useRef(false);
  const askAboutProcesses = (
    action: "stop" | "archive",
    sessions: readonly SessionPayload[],
    run: (
      processes?: SessionProcessPolicy,
      readErrors?: ReadonlyMap<string, unknown>,
      trees?: SessionTrees
    ) => void
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
    void readSessionChoiceTargets(queryClient, action, sessions)
      .then(({ current, trees }) => {
        const runChecked = (processes?: SessionProcessPolicy, withChildren = false) =>
          run(processes, current.errors, withChildren ? trees : undefined);
        const scope = sessionChildrenScope("archive", current.sessions, trees);
        if (sessionsRunningProcesses(current.sessions) === 0 && scope.members.length === 0) {
          runChecked();
          return;
        }
        setArchiveChildren(true);
        setProcessChoice({ action, sessions: current.sessions, scope, run: runChecked });
      })
      .catch(error => reportActionError(action, error))
      .finally(() => {
        checkingProcesses.current = false;
      });
  };

  /** Retry failed IDs while preserving successful results across the dialog lifetime. */
  const confirmDeleteMany = async (readErrors?: ReadonlyMap<string, unknown>) => {
    if (!deleteTarget) return;
    const failedIds = new Set(
      batchResults.flatMap(result => (result.status === "failed" ? [result.id] : []))
    );
    const targets =
      failedIds.size > 0
        ? deleteTarget.sessions.filter(session => failedIds.has(session.id))
        : deleteTarget.sessions;
    const results = await runBatch(
      "delete",
      targets,
      batchResults,
      deletePolicy.current,
      readErrors,
      includedDeleteTrees
    );
    if (!results) return;
    const failures = results.filter(result => result.status === "failed");
    deleteTarget.onSelectionChange?.(failures.map(result => result.id));
    if (failures.length === 0) {
      setDeleteTarget(null);
      reportDeletedSessions(results);
    }
  };

  /** Keep single-row deletion behavior while routing selection batches through result tracking. */
  const performDelete = (
    processes?: SessionProcessPolicy,
    readErrors?: ReadonlyMap<string, unknown>
  ) => {
    if (!deleteSession || remove.isPending || batchRunning.current) return;
    if (processes) deletePolicy.current = processes;
    if (deleteTarget?.bulk) {
      void confirmDeleteMany(readErrors);
      return;
    }
    const { id } = deleteSession;
    const policy = deletePolicy.current ? { processes: deletePolicy.current } : {};
    remove.mutate(
      { ...deleteSession, ...policy, ...descendantsOf(deleteSession, includedDeleteTrees) },
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
    const confirmation = ++deleteConfirmation.current;
    const failedIds = new Set<string>();
    for (const result of batchResults) {
      if (result.status === "failed") failedIds.add(result.id);
    }
    const targets =
      deleteTarget.bulk && failedIds.size > 0
        ? deleteTarget.sessions.filter(session => failedIds.has(session.id))
        : deleteTarget.sessions;
    void readCurrentSessions(queryClient, targets, deleteTarget.bulk)
      .then(current => {
        if (confirmation !== deleteConfirmation.current) return;
        if (!deleteTarget.bulk && current.errors.size) throw current.errors.values().next().value;
        // A fresh nonzero count must be answered in the now-updated dialog.
        if (!processes && !deletePolicy.current && sessionsRunningProcesses(current.sessions) > 0) {
          if (failedIds.size > 0) {
            reportDeletedSessions(batchResults);
            setDeleteTarget(target => (target ? { ...target, sessions: targets } : null));
            setBatchResults([]);
          }
          return;
        }
        performDelete(processes, current.errors);
      })
      .catch(error => {
        if (confirmation === deleteConfirmation.current) reportActionError("delete", error);
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

  const { archiveChoice, processTargets } = sessionArchiveChoiceState(
    processChoice,
    archiveChildren,
    setArchiveChildren
  );

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
        void askAboutProcesses("archive", [session], (processes, _readErrors, trees) =>
          archive.mutate(archiveTarget(session, processes, trees), {
            onError: error => reportActionError("archive", error),
            onSuccess: () => void notifyArchived(session, trees),
          })
        ),
      onUnarchive: session =>
        unarchive.mutate(session, { onError: error => reportActionError("unarchive", error) }),
      onDelete: session => {
        if (pendingAction === null && !batchRunning.current) {
          deleteConfirmation.current++;
          deletePolicy.current = undefined;
          setDeleteChildren(false);
          setBatchResults([]);
          setDeleteTarget({ sessions: [session], bulk: false, openedAt: Date.now() });
        }
      },
      onStopMany: sessions => actOnMany("stop", sessions),
      onArchiveMany: sessions => actOnMany("archive", sessions),
      onUnarchiveMany: sessions => actOnMany("unarchive", sessions),
      ...later.handlers,
      onAdopt: reparent.adopt,
      onMakeTopLevel: reparent.detach,
      onDeleteMany: (sessions, onSelectionChange) => {
        if (pendingAction === null && !batchRunning.current && sessions.length > 0) {
          deleteConfirmation.current++;
          deletePolicy.current = undefined;
          setDeleteChildren(false);
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
      childSessions: sessionChildrenChoice(deleteScope, deleteChildren, setDeleteChildren, {
        pending: !deleteRead.settled,
        onRetry: deleteRead.onRetry,
      }),
      onOpenChange: open => {
        if (!open && deleteTarget && !remove.isPending && !batchRunning.current) {
          deleteConfirmation.current++;
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
      processCount: sessionsRunningProcesses(processTargets),
      ports: processTargets.flatMap(session => sessionProcessPorts(session)),
      childSessions: archiveChoice,
      onOpenChange: open => {
        if (!open) setProcessChoice(null);
      },
      onChoose: processes => {
        const choice = processChoice;
        setProcessChoice(null);
        choice?.run(processes, archiveChoice?.included ?? false);
      },
    },
    laterDialog: later.laterDialog,
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
