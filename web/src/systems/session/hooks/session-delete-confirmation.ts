import type { Dispatch, SetStateAction } from "react";
import type { QueryClient } from "@tanstack/react-query";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { readCurrentSessions, type SessionBatchResult } from "../lib/session-batch";
import { sessionsRunningProcesses } from "../lib/session-processes";
import { sessionChildrenScope } from "../lib/session-tree";
import type { SessionPayload } from "../types";
import { reportActionError, reportDeletedSessions } from "./session-lifecycle-helpers";
import type { SessionDeleteTarget } from "./use-session-delete-target";

interface ConfirmSessionDeleteOptions {
  queryClient: QueryClient;
  target: SessionDeleteTarget;
  batchResults: readonly SessionBatchResult[];
  deletePolicy?: SessionProcessPolicy;
  processes?: SessionProcessPolicy;
  isCurrent: () => boolean;
  setBatchResults: (results: readonly SessionBatchResult[]) => void;
  setTarget: Dispatch<SetStateAction<SessionDeleteTarget | null>>;
  performDelete: (
    processes?: SessionProcessPolicy,
    readErrors?: ReadonlyMap<string, unknown>
  ) => void;
}

/** Re-read delete targets before mutation, asking again if their process state changed. */
export async function confirmSessionDelete({
  queryClient,
  target,
  batchResults,
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
    const current = await readCurrentSessions(queryClient, targets, target.bulk);
    if (!isCurrent()) return;
    if (!target.bulk && current.errors.size) throw current.errors.values().next().value;
    if (!processes && !deletePolicy && sessionsRunningProcesses(current.sessions) > 0) {
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
              processCount: sessionsRunningProcesses(displaySessions),
              childrenScope: sessionChildrenScope("delete", displaySessions, value.trees),
            }
          : null
      );
      return;
    }
    performDelete(processes, current.errors);
  } catch (error) {
    if (isCurrent()) reportActionError("delete", error);
  }
}
