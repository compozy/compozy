import type { AgentInstancePayload } from "@/systems/agent";

import type { SessionPayload } from "../types";

/**
 * Where a superseded session's conversation went. Superseded derives from the instance pointer,
 * never a marker: a session of an instance that isn't its current main moved on. `instance_id`
 * joins the session DTO with rotation; until then no session reads as superseded.
 */
export function sessionSuccessorId(
  session: SessionPayload & { instance_id?: string },
  instances: readonly AgentInstancePayload[]
): string | null {
  if (!session.instance_id) return null;
  const main = instances.find(instance => instance.id === session.instance_id)?.main_session_id;
  return main && main !== session.id ? main : null;
}
