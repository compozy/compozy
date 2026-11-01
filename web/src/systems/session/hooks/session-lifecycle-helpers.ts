import { toast } from "@compozy/ui";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionBatchResult } from "../lib/session-batch";
import { isSessionUnavailable } from "../adapters/session-api-errors";
import {
  sessionChildrenChoice,
  type SessionChildrenScope,
  type SessionTrees,
} from "../lib/session-tree";
import type { SessionPayload } from "../types";
import type { SessionLifecycleAction } from "./session-lifecycle-types";

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

/** The process dialog's child choice, and the sessions its process count covers. */
export function sessionArchiveChoiceState(
  processChoice: { sessions: readonly SessionPayload[]; scope?: SessionChildrenScope } | null,
  included: boolean,
  onIncludedChange: (included: boolean) => void
) {
  const scope = processChoice?.scope;
  const archiveChoice = scope
    ? sessionChildrenChoice(scope, included, onIncludedChange)
    : undefined;
  const processTargets = [
    ...(processChoice?.sessions ?? []),
    ...(archiveChoice?.included ? (scope?.reached ?? []) : []),
  ];
  return { archiveChoice, processTargets };
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
