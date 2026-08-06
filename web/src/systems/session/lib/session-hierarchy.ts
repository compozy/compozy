import type { SessionPayload } from "../types";

export interface SessionListTree {
  /** Sessions rendered as top-level rows: no parent, or parent not on this page. */
  roots: SessionPayload[];
  /** Direct children keyed by parent session id, in the incoming list order. */
  childrenByParent: Map<string, SessionPayload[]>;
}

/**
 * Nest every session whose creation parent is present in the same page under
 * that parent. A child whose parent is not loaded stays a root so nothing
 * silently vanishes behind pagination.
 */
export function buildSessionTree(sessions: readonly SessionPayload[]): SessionListTree {
  const byId = new Set(sessions.map(session => session.id));
  const roots: SessionPayload[] = [];
  const childrenByParent = new Map<string, SessionPayload[]>();
  for (const session of sessions) {
    const parentId = session.lineage?.parent_session_id ?? "";
    if (parentId !== "" && parentId !== session.id && byId.has(parentId)) {
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
  const stack = [...(childrenByParent.get(rootId) ?? [])];
  while (stack.length > 0) {
    const next = stack.shift();
    if (!next) break;
    collected.push(next);
    const grandchildren = childrenByParent.get(next.id);
    if (grandchildren) stack.unshift(...grandchildren);
  }
  return collected;
}

export type ChildSessionSignalTone = "danger" | "warning" | "accent";

/** Most urgent child state, so a collapsed thread never hides an escalation. */
export function childSessionSignalTone(
  children: readonly SessionPayload[]
): ChildSessionSignalTone | null {
  let tone: ChildSessionSignalTone | null = null;
  for (const child of children) {
    const badge = child.badge;
    if (badge === "failed" || badge === "unhealthy") return "danger";
    if (badge === "waiting-for-auth" || badge === "hung") {
      tone = "warning";
    } else if (badge === "running" && tone !== "warning") {
      tone = "accent";
    }
  }
  return tone;
}
