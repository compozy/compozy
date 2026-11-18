interface SessionDisplayIdentity {
  id: string;
  name?: string | null;
  agent_name?: string | null;
  /** Why the daemon created the session; absent reads as operator-created. */
  type?: string | null;
  /** The always-on agent instance that owns the session. */
  instance_id?: string | null;
}

export const UNTITLED_SESSION_TITLE = "New session";

/**
 * An operator's untitled session reads as "New session" until it earns a title. A session an
 * automation, an always-on agent, or another agent started keeps its origin name instead,
 * even when that name is the agent's, so the operator can tell who created it.
 */
export function getSessionDisplayTitle(session: SessionDisplayIdentity): string {
  const persistedName = session.name?.trim();
  const agentName = session.agent_name?.trim();
  const named = persistedName && persistedName !== session.id ? persistedName : "";
  if (!hasNonOperatorOrigin(session)) {
    return named && named !== agentName ? named : UNTITLED_SESSION_TITLE;
  }
  return named || agentName || UNTITLED_SESSION_TITLE;
}

function hasNonOperatorOrigin(session: SessionDisplayIdentity): boolean {
  if (session.instance_id?.trim()) return true;
  const type = session.type?.trim();
  return Boolean(type) && type !== "user";
}
