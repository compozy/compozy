import { sessionBadgeSignal } from "./session-badge";
import type { SessionPayload } from "../types";

export interface SessionListTree {
  /** Sessions rendered as top-level rows: no loaded parent, or part of a parent cycle. */
  roots: SessionPayload[];
  /** Direct children keyed by parent session id, in the incoming list order. */
  childrenByParent: Map<string, SessionPayload[]>;
}

/**
 * Nest every session whose creation parent is present in the same page under
 * that parent. Missing parents and cycle participants stay roots so nothing
 * silently vanishes behind pagination or malformed lineage.
 */
export function buildSessionTree(sessions: readonly SessionPayload[]): SessionListTree {
  const parentById = new Map(
    sessions.map(session => [session.id, session.lineage?.parent_session_id ?? ""])
  );
  const cycleParticipants = findCycleParticipants(parentById);
  const roots: SessionPayload[] = [];
  const childrenByParent = new Map<string, SessionPayload[]>();
  for (const session of sessions) {
    const parentId = session.lineage?.parent_session_id ?? "";
    if (parentId !== "" && parentById.has(parentId) && !cycleParticipants.has(session.id)) {
      const siblings = childrenByParent.get(parentId);
      if (siblings) {
        siblings.push(session);
      } else {
        childrenByParent.set(parentId, [session]);
      }
      continue;
    }
    roots.push(session);
  }
  return { roots, childrenByParent };
}

function findCycleParticipants(parentById: ReadonlyMap<string, string>): Set<string> {
  const cycleParticipants = new Set<string>();
  const resolved = new Set<string>();

  for (const sessionId of parentById.keys()) {
    if (resolved.has(sessionId)) continue;

    const path: string[] = [];
    const pathIndexes = new Map<string, number>();
    let currentId = sessionId;
    while (parentById.has(currentId) && !resolved.has(currentId)) {
      const cycleStart = pathIndexes.get(currentId);
      if (cycleStart !== undefined) {
        for (const cycleId of path.slice(cycleStart)) cycleParticipants.add(cycleId);
        break;
      }

      pathIndexes.set(currentId, path.length);
      path.push(currentId);
      const parentId = parentById.get(currentId) ?? "";
      if (parentId === "" || !parentById.has(parentId)) break;
      currentId = parentId;
    }
    for (const pathId of path) resolved.add(pathId);
  }

  return cycleParticipants;
}

/**
 * Every descendant of one thread root, flattened in depth-first list order.
 * The sidebar renders one visual nesting level, so deep chains stay inside
 * their root's thread instead of drifting into unreadable indentation.
 */
export function collectThreadSessions(
  rootId: string,
  childrenByParent: SessionListTree["childrenByParent"]
): SessionPayload[] {
  const collected: SessionPayload[] = [];
  const stack = [...(childrenByParent.get(rootId) ?? [])].reverse();
  while (stack.length > 0) {
    const next = stack.pop();
    if (!next) break;
    collected.push(next);
    const grandchildren = childrenByParent.get(next.id);
    if (grandchildren) {
      for (let index = grandchildren.length - 1; index >= 0; index -= 1) {
        stack.push(grandchildren[index]!);
      }
    }
  }
  return collected;
}

/**
 * Keep matching descendants and their ancestor path in one reverse tree pass.
 * A matching root keeps its complete thread, matching the unfiltered catalog.
 */
export function filterThreadSessions(
  root: SessionPayload,
  childrenByParent: SessionListTree["childrenByParent"],
  matches: (session: SessionPayload) => boolean
): SessionPayload[] | null {
  const descendants = collectThreadSessions(root.id, childrenByParent);
  if (matches(root)) return descendants;

  const visibleIds = new Set<string>();
  for (let index = descendants.length - 1; index >= 0; index -= 1) {
    const session = descendants[index]!;
    const hasVisibleChild = (childrenByParent.get(session.id) ?? []).some(child =>
      visibleIds.has(child.id)
    );
    if (matches(session) || hasVisibleChild) visibleIds.add(session.id);
  }
  if (visibleIds.size === 0) return null;
  return descendants.filter(session => visibleIds.has(session.id));
}

export type ChildSessionSignalTone = "danger" | "warning" | "accent";

/**
 * Most urgent child state, so a collapsed thread never hides an escalation.
 * Urgency is read from the one badge dictionary rather than a local badge list,
 * so every member of the needs-you class escalates and runtime-health warnings
 * stay a rank below it. `done` deliberately raises nothing: finished-unseen work
 * is an inbox item, not an escalation.
 */
export function childSessionSignalTone(
  children: readonly SessionPayload[]
): ChildSessionSignalTone | null {
  let tone: ChildSessionSignalTone | null = null;
  for (const child of children) {
    const signal = sessionBadgeSignal(child.badge);
    if (signal.attention === "needs-you") return "danger";
    if (signal.tone === "warning") {
      tone = "warning";
    } else if (signal.label === "running" && tone !== "warning") {
      tone = "accent";
    }
  }
  return tone;
}
