import type { AgentInstancePayload } from "@/systems/agent";

import type { SessionLineagePayload } from "../types";

/** Lineage kinds that start a new top-level session: their source is provenance, not an owner. */
const NEW_TOP_LEVEL: ReadonlySet<string | undefined> = new Set(["fork", "continue"]);

/**
 * The always-on agent a session belongs to: its main session, or a side chat, child, or subagent
 * under it. The session itself wins over its parent, and the parent over the lineage root, so a
 * main session spawned under another agent's main still reads as its own agent's. A fork or
 * continuation of a main is a new session of its own and belongs to no agent unless it is a main.
 *
 * ponytail: matches the parent and root only; a grandchild whose root moved with a reparent is not
 * found, since the payload carries no ancestor list.
 */
export function alwaysOnOwnerOf(
  instances: readonly AgentInstancePayload[],
  session: {
    id: string;
    lineage?: Pick<SessionLineagePayload, "kind" | "parent_session_id" | "root_session_id"> | null;
  }
): AgentInstancePayload | undefined {
  const byMain = new Map(instances.map(instance => [instance.main_session_id, instance]));
  const lineage = session.lineage;
  const ancestors = NEW_TOP_LEVEL.has(lineage?.kind)
    ? []
    : [lineage?.parent_session_id?.trim(), lineage?.root_session_id?.trim()];
  for (const id of [session.id, ...ancestors]) {
    const owner = id ? byMain.get(id) : undefined;
    if (owner) return owner;
  }
  return undefined;
}
