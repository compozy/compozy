/**
 * Which selected sessions a lifecycle verb applies to. Pure, so the menus that count and hide
 * verbs never reach the batch runner, its query cache reads, or the network.
 */
import type { SessionPayload } from "../types";
import { isSubagentSession } from "./session-hierarchy";
import { sessionIsInLater } from "./session-later";

export type BatchVerb = "stop" | "archive" | "unarchive" | "later" | "back-from-later";

/** The skip reason of a subagent session, which follows its parent; `batchSkipNote` words it. */
export const BATCH_CHILD_SKIP = "child";

/** Why a selected session sits a verb out; `null` when the verb applies to it. */
export function batchSkipReason(action: BatchVerb, session: SessionPayload): string | null {
  const archived = session.archived_at !== null;
  if (action === "stop") {
    if (session.state !== "active" && session.state !== "starting") return "not running";
    return archived ? "archived" : null;
  }
  // Subagent sessions archive, unarchive, and move to or back from Later with
  // their parent (ADR-005), never on their own.
  if (isSubagentSession(session)) return BATCH_CHILD_SKIP;
  if (action === "later") {
    if (archived) return "archived";
    return sessionIsInLater(session) ? "already in Later" : null;
  }
  if (action === "back-from-later") return sessionIsInLater(session) ? null : "not in Later";
  if (action === "archive") return archived ? "already archived" : null;
  return archived ? null : "not archived";
}
