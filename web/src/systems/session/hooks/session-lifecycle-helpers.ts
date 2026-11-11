import { readArchivedDescendants } from "@/systems/session/lib/session-tree";
import { toast } from "@compozy/ui";
import type { QueryClient } from "@tanstack/react-query";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionBatchResult } from "../lib/session-batch";
import { isSessionUnavailable } from "../adapters/session-api-errors";
import {
  sessionChildrenChoice,
  sessionChildrenScope,
  readSessionChoiceTargets,
  SessionTreeReadError,
  type SessionTrees,
} from "../lib/session-tree";
import type { SessionPayload } from "../types";
import type { SessionLifecycleAction, SessionProcessChoice } from "./session-lifecycle-types";
import { sessionsRunningProcesses } from "../lib/session-processes";

/**
 * Archive always uses the daemon's stop-first operation: a live session stops
 * first, and a stopped parent whose subagent session is still live (until the
 * reaper stops it) archives too. A fully stopped family archives as before.
 */
export function archiveTarget(
  session: SessionPayload,
  processes?: SessionProcessPolicy,
  trees?: SessionTrees
) {
  return {
    ...session,
    stop: true,
    ...(processes ? { processes } : {}),
    ...descendantsOf(session, trees),
  };
}

/** Trees mean the operator included children, even if this target's read found none. */
export function descendantsOf(session: SessionPayload, trees?: SessionTrees) {
  return trees ? { descendants: trees.get(session.id) ?? [] } : {};
}

/** Its root target's tree may have taken a covered target, even if that root then failed. */
export async function settleTreeTarget(
  action: "archive" | "delete",
  covered: boolean,
  run: () => Promise<void>
): Promise<void> {
  try {
    await run();
  } catch (error) {
    if (action !== "delete" || !covered || !isSessionUnavailable(error)) throw error;
  }
}

/** A host callback failing does not undo an archive the daemon confirmed. */
export async function notifyArchivedSessions(
  queryClient: QueryClient,
  session: SessionPayload,
  trees: SessionTrees | undefined,
  onSuccess?: (session: SessionPayload) => void | Promise<void>
): Promise<void> {
  const children = await readArchivedDescendants(queryClient, trees?.get(session.id) ?? []);
  const results = await Promise.allSettled(
    [session, ...children].map(target => onSuccess?.(target))
  );
  for (const result of results) {
    if (result.status === "rejected") reportActionError("archive", result.reason);
  }
}

/** The process dialog's child choice, and the sessions its process count covers. */
export function sessionArchiveChoiceState(
  processChoice: SessionProcessChoice | null,
  included: boolean,
  onIncludedChange: (included: boolean) => void
) {
  const scope = processChoice?.scope;
  const archiveChoice = scope
    ? sessionChildrenChoice(scope, included, onIncludedChange, processChoice?.read)
    : undefined;
  const processTargets = [
    ...(processChoice?.sessions ?? []),
    ...(archiveChoice?.included ? (scope?.reached ?? []) : []),
  ];
  return { archiveChoice, processTargets };
}

/** An unreadable tree remains a retryable confirmation, never an empty tree. */
export async function loadSessionProcessChoice(
  queryClient: QueryClient,
  action: "stop" | "archive",
  sessions: readonly SessionPayload[],
  run: (
    processes?: SessionProcessPolicy,
    errors?: ReadonlyMap<string, unknown>,
    trees?: SessionTrees
  ) => void,
  onChoice: (choice: SessionProcessChoice | null) => void,
  isCurrent: () => boolean
): Promise<void> {
  try {
    const { current, trees } = await readSessionChoiceTargets(queryClient, action, sessions);
    if (!isCurrent()) return;
    const runChecked = (processes?: SessionProcessPolicy, withChildren = false) =>
      run(processes, current.errors, withChildren ? trees : undefined);
    const scope = sessionChildrenScope("archive", current.sessions, trees);
    if (sessionsRunningProcesses(current.sessions) === 0 && scope.members.length === 0) {
      onChoice(null);
      runChecked();
    } else {
      onChoice({ action, sessions: current.sessions, scope, run: runChecked });
    }
  } catch (error) {
    if (!isCurrent()) return;
    if (action !== "archive" || !(error instanceof SessionTreeReadError)) {
      reportActionError(action, error);
      return;
    }
    const failed: SessionProcessChoice = {
      action,
      sessions,
      scope: sessionChildrenScope("archive", sessions, new Map()),
      read: { pending: true },
      run: () => {},
    };
    onChoice({
      ...failed,
      read: {
        pending: true,
        onRetry: () => {
          onChoice(failed);
          void loadSessionProcessChoice(queryClient, action, sessions, run, onChoice, isCurrent);
        },
      },
    });
  }
}

/** Preserve the daemon message, falling back only when the failure has no useful text. */
export function reportActionError(action: SessionLifecycleAction, error: unknown): void {
  const fallback = `Failed to ${action} session.`;
  toast.error(error instanceof Error && error.message ? error.message : fallback);
}

/** Report all successful deletes once, when the result dialog closes. */
export function reportDeletedSessions(results: readonly SessionBatchResult[]): void {
  const count = results.filter(result => result.status === "done").length;
  if (count > 0) toast.success(`${count} ${count === 1 ? "session" : "sessions"} deleted`);
}
