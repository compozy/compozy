import { useAgentInstances } from "@/systems/agent";

import type { SessionPayload } from "../types";

/** The stays-on agent whose main session spawned this session, if any. */
export function useSessionCreator(session: Pick<SessionPayload, "id" | "lineage">) {
  const parents = new Set([session.lineage?.parent_session_id, session.lineage?.root_session_id]);
  return useAgentInstances().find(
    instance =>
      instance.main_session_id != null &&
      instance.main_session_id !== session.id &&
      parents.has(instance.main_session_id)
  );
}
