import type { AgentInstancePayload } from "@/systems/agent";

import type { SessionPayload } from "../types";

/**
 * The always-on agent a session belongs to: its main session, or a side chat, child, or subagent
 * under it. The session itself wins over its parent, and the parent over the lineage root, so a
 * main session spawned under another agent's main still reads as its own agent's.
 */
export function alwaysOnOwnerOf(
  instances: readonly AgentInstancePayload[],
  session: Pick<SessionPayload, "id" | "lineage">
): AgentInstancePayload | undefined {
  const candidates = [
    session.id,
    session.lineage?.parent_session_id?.trim(),
    session.lineage?.root_session_id?.trim(),
  ];
  for (const id of candidates) {
    if (!id) continue;
    const owner = instances.find(instance => instance.main_session_id === id);
    if (owner) return owner;
  }
  return undefined;
}
