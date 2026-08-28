import type { SessionPayload } from "@/systems/session";

/** Latest `created_at` in the live catalog; ties break by id. Archived rows never win. */
export function pickLastCreatedSession(sessions: readonly SessionPayload[]): SessionPayload | null {
  let latest: SessionPayload | null = null;
  for (const session of sessions) {
    if (session.archived_at) continue;
    if (latest === null) {
      latest = session;
      continue;
    }
    if (session.created_at > latest.created_at) {
      latest = session;
      continue;
    }
    if (session.created_at === latest.created_at && session.id > latest.id) {
      latest = session;
    }
  }
  return latest;
}
