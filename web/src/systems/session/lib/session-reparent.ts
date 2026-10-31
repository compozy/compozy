import type { SessionPayload } from "../types";
import { isSubagentSession } from "./session-hierarchy";
import { isUserControllableSession } from "./session-running";

/**
 * Why `source` cannot become `target`'s child, or `null` when the drop may be
 * offered. The daemon owns the decision and re-validates every rule; this only
 * keeps refused drops from looking possible.
 */
export function sessionAdoptRefusal(source: SessionPayload, target: SessionPayload): string | null {
  if (source.id === target.id) return "A session can't be its own child.";
  if (!isUserControllableSession(source) || (source.lineage?.parent_session_id ?? "") !== "") {
    return "Only a top-level session can move under another session.";
  }
  if (isSubagentSession(source) || isSubagentSession(target)) {
    return "Subagent sessions stay with their parent.";
  }
  if (!isUserControllableSession(target)) return "Only a user session can hold another session.";
  if (source.archived_at !== null || target.archived_at !== null) {
    return "Unarchive both sessions first.";
  }
  if (source.workspace_id !== target.workspace_id || source.profile_id !== target.profile_id) {
    return "Both sessions must belong to one project and profile.";
  }
  if (target.lineage?.root_session_id === source.id) {
    return "A session can't move under one of its own children.";
  }
  return null;
}

/** A child the operator nested (provenance lineage) can be made top-level again. */
export function sessionCanMakeTopLevel(session: SessionPayload): boolean {
  return (
    isUserControllableSession(session) &&
    session.archived_at === null &&
    session.lineage?.kind === "provenance" &&
    (session.lineage.parent_session_id ?? "") !== ""
  );
}

/**
 * A stopped spawned worker the operator can take over as a normal session. The
 * daemon ends its spawn governance (TTL, parent auto-stop, permission narrowing);
 * subagents stay with their parent, and a running worker must stop first.
 */
export function sessionCanTakeOver(session: SessionPayload): boolean {
  return (
    session.type === "spawned" &&
    !isSubagentSession(session) &&
    session.lineage?.kind === "spawn" &&
    session.state === "stopped" &&
    session.archived_at === null
  );
}
