import { toast } from "@compozy/ui";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionBatchResult } from "../lib/session-batch";
import type { SessionTrees } from "../lib/session-tree";
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
  trees?: SessionTrees,
) {
  return {
    ...session,
    stop: true,
    ...(processes ? { processes } : {}),
    ...descendantsOf(session, trees),
  };
}

/** Only an included, nonempty tree opts in, so every other request keeps its meaning. */
export function descendantsOf(session: SessionPayload, trees?: SessionTrees) {
  const descendants = trees?.get(session.id);
  return descendants?.length ? { descendants } : {};
}

/** Preserve the daemon message, falling back only when the failure has no useful text. */
export function reportActionError(action: SessionLifecycleAction, error: unknown): void {
  const fallback = `Failed to ${action} session.`;
  toast.error(error instanceof Error && error.message ? error.message : fallback);
}

/** Report all successful deletes once, when the result dialog closes. */
export function reportDeletedSessions(results: readonly SessionBatchResult[]): void {
  const count = results.filter((result) => result.status === "done").length;
  if (count > 0) toast.success(`${count} ${count === 1 ? "session" : "sessions"} deleted`);
}
