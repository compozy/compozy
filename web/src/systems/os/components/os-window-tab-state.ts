import type { SessionPayload } from "@/systems/session";

export type OsWindowTabState = "running" | "needs-input" | "attention" | "quiet" | null;

/** Session badge → deck vocabulary: accent marks attention only (BR-14). */
export function sessionTabState(session: SessionPayload | undefined): OsWindowTabState {
  if (!session) return "quiet";
  switch (session.badge) {
    case "running":
      return "running";
    case "waiting-for-auth":
      return "needs-input";
    case "hung":
    case "failed":
    case "unhealthy":
      return "attention";
    default:
      return "quiet";
  }
}
