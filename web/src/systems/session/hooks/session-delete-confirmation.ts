import type { Dispatch, SetStateAction } from "react";
import type { QueryClient } from "@tanstack/react-query";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { readCurrentSessions, type SessionBatchResult } from "../lib/session-batch";
import { sessionsRunningProcesses } from "../lib/session-processes";
import { readSessionTrees, sessionChildrenScope, type SessionTrees } from "../lib/session-tree";
import { reportActionError, reportDeletedSessions } from "./session-lifecycle-helpers";
import type { SessionDeleteTarget } from "./use-session-delete-target";

interface ConfirmSessionDeleteOptions {
  queryClient: QueryClient;
  target: SessionDeleteTarget;
  batchResults: readonly SessionBatchResult[];
  includeChildren: boolean;
  deletePolicy?: SessionProcessPolicy;
  processes?: SessionProcessPolicy;
  isCurrent: () => boolean;
  setBatchResults: (results: readonly SessionBatchResult[]) => void;
  setTarget: Dispatch<SetStateAction<SessionDeleteTarget | null>>;
  performDelete: (
    processes?: SessionProcessPolicy,
    readErrors?: ReadonlyMap<string, unknown>,
    trees?: SessionTrees
  ) => void;
}

/** Re-read delete targets (and included child trees) before mutation, asking again if scope grew. */
export async function confirmSessionDelete({
  queryClient,
  target,
  batchResults,
  includeChildren,
  deletePolicy,
  processes,
  isCurrent,
  setBatchResults,
  setTarget,
  performDelete,
}: ConfirmSessionDeleteOptions): Promise<void> {
  const failedIds = new Set(
    batchResults.flatMap(result => (result.status === "failed" ? [result.id] : []))
  );
  const targets =
    target.bulk && failedIds.size > 0
      ? target.sessions.filter(session => failedIds.has(session.id))
      : target.sessions;
  try {
    const [current, treeRead] = await Promise.all([
      readCurrentSessions(queryClient, targets, target.bulk),
      includeChildren ? readSessionTrees(queryClient, targets) : Promise.resolve(null),
    ]);
    if (!isCurrent()) return;
    if (!target.bulk && current.errors.size) throw current.errors.values().next().value;

    const mergedTrees = new Map(target.trees);
    const mergedTreeErrors = new Map(target.treeErrors);
    if (treeRead) {
      for (const session of targets) mergedTreeErrors.delete(session.id);
      for (const [id, tree] of treeRead.trees) mergedTrees.set(id, tree);
      for (const [id, error] of treeRead.errors) mergedTreeErrors.set(id, error);
    }

    const previousScope = sessionChildrenScope("delete", targets, target.trees);
    const nextScope = sessionChildrenScope("delete", targets, mergedTrees);
    const previousMemberIds = new Set(previousScope.members.map(child => child.id));
    const previousReachedIds = new Set(previousScope.reached.map(child => child.id));
    const childrenGrew =
      treeRead !== null &&
      (nextScope.members.some(child => !previousMemberIds.has(child.id)) ||
        nextScope.reached.some(child => !previousReachedIds.has(child.id)));
    const treeFailed = treeRead !== null && mergedTreeErrors.size > 0;
    const includedProcesses = includeChildren ? sessionsRunningProcesses(nextScope.reached) : 0;
    const needsProcessAnswer =
      !processes &&
      !deletePolicy &&
      sessionsRunningProcesses(current.sessions) + includedProcesses > 0;

    if (treeFailed || childrenGrew || needsProcessAnswer) {
      const currentById = new Map(current.sessions.map(session => [session.id, session]));
      const currentTargets = targets.map(session => currentById.get(session.id) ?? session);
      const displaySessions =
        failedIds.size > 0
          ? currentTargets
          : target.sessions.map(session => currentById.get(session.id) ?? session);
      if (failedIds.size > 0) {
        reportDeletedSessions(batchResults);
        setBatchResults([]);
      }
      setTarget(value =>
        value
          ? {
              ...value,
              sessions: Object.freeze(displaySessions),
              trees: mergedTrees,
              treeErrors: mergedTreeErrors,
              processCount: sessionsRunningProcesses(displaySessions),
              childrenScope: sessionChildrenScope("delete", displaySessions, mergedTrees),
              treeReadPending: false,
            }
          : null
      );
      return;
    }

    const confirmedTrees =
      includeChildren && nextScope.members.length > 0 ? mergedTrees : undefined;
    performDelete(processes, current.errors, confirmedTrees);
  } catch (error) {
    if (isCurrent()) reportActionError("delete", error);
  }
}
