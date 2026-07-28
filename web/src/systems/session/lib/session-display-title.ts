interface SessionDisplayIdentity {
  id: string;
  name?: string | null;
  agent_name?: string | null;
}

export const UNTITLED_SESSION_TITLE = "New session";

export function getSessionDisplayTitle(session: SessionDisplayIdentity): string {
  const persistedName = session.name?.trim();
  const agentName = session.agent_name?.trim();
  const isMeaningfulIdentity =
    persistedName && persistedName !== session.id && persistedName !== agentName;
  return isMeaningfulIdentity ? persistedName : UNTITLED_SESSION_TITLE;
}
