import { toast } from "sonner";
import type { SessionPayload } from "../types";
import { sessionBatchFailureMessage, type SessionBatchResult } from "./session-batch";
import { sessionIsInLater } from "./session-later";
import { getSessionDisplayTitle } from "./session-display-title";
import { isSubagentSession } from "./session-hierarchy";

export type BatchVerb = "stop" | "archive" | "unarchive" | "later" | "back-from-later";

const sessionCount = (count: number) => `${count} ${count === 1 ? "session" : "sessions"}`;

const CHILD_SKIP = "child";

/** Why a selected session sits a verb out; `null` when the verb applies to it. */
export function batchSkipReason(action: BatchVerb, session: SessionPayload): string | null {
  const archived = session.archived_at !== null;
  if (action === "stop") {
    if (session.state !== "active" && session.state !== "starting") return "not running";
    return archived ? "archived" : null;
  }
  // Subagent sessions archive, unarchive, and move to or back from Later with
  // their parent (ADR-005), never on their own.
  if (isSubagentSession(session)) return CHILD_SKIP;
  if (action === "later") {
    if (archived) return "archived";
    return sessionIsInLater(session) ? "already in Later" : null;
  }
  if (action === "back-from-later") return sessionIsInLater(session) ? null : "not in Later";
  if (action === "archive") return archived ? "already archived" : null;
  return archived ? null : "not archived";
}

export function batchVerbPast(action: BatchVerb): string {
  if (action === "later") return "moved to Later";
  if (action === "back-from-later") return "back from Later";
  return action === "stop" ? "stopped" : `${action}d`;
}

/** "Couldn't move 2 sessions to Later". */
function batchFailureTitle(action: BatchVerb, count: string): string {
  if (action === "later") return `Couldn't move ${count} to Later`;
  if (action === "back-from-later") return `Couldn't bring ${count} back from Later`;
  return `Couldn't ${action} ${count}`;
}

/** The selection a verb skipped, by reason: "2 skipped: 1 already archived, 1 not running". */
export function batchSkipNote(skipped: ReadonlyMap<string, number>): string | undefined {
  const total = [...skipped.values()].reduce((sum, count) => sum + count, 0);
  if (total === 0) return undefined;
  const reasons = [...skipped]
    .map(([reason, count]) =>
      reason !== CHILD_SKIP
        ? `${count} ${reason}`
        : `${count} ${count === 1 ? "child session follows its parent" : "child sessions follow their parent"}`
    )
    .join(", ");
  return `${total} skipped: ${reasons}.`;
}

/**
 * Non-delete batches have no result dialog, so one toast carries the outcome:
 * the success count, each failed session by name with its own error, and the
 * selected sessions the verb does not apply to, by reason.
 */
export function reportBatch(
  action: BatchVerb,
  results: readonly SessionBatchResult[],
  sessions: readonly SessionPayload[],
  skipped: ReadonlyMap<string, number>
): void {
  const verb = batchVerbPast(action);
  const done = results.filter(result => result.status === "done").length;
  const failures = results.filter(result => result.status === "failed");
  const skipNote = batchSkipNote(skipped);
  if (failures.length === 0) {
    toast.success(`${sessionCount(done)} ${verb}`, { description: skipNote });
    return;
  }
  const byId = new Map(sessions.map(session => [session.id, session]));
  const description = failures
    .map(result => {
      const session = byId.get(result.id);
      const title = session ? getSessionDisplayTitle(session) : result.id;
      return `${title}: ${result.error ?? sessionBatchFailureMessage(action)}`;
    })
    .join("; ")
    .concat(skipNote ? ` ${skipNote}` : "");
  const title =
    done > 0
      ? `${sessionCount(done)} ${verb}, ${failures.length} failed`
      : batchFailureTitle(action, sessionCount(failures.length));
  toast.error(title, { description });
}
