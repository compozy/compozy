import { useQueries } from "@tanstack/react-query";

import { SessionNotFoundError, sessionOwnerOptions } from "@/systems/session";

/**
 * The evidence sessions the daemon no longer knows (S2). Only a 404 marks a session unavailable;
 * any other failure keeps it linkable, because the session may still exist.
 */
export function useUnavailableEvidenceSessions(ids: readonly string[]): readonly string[] {
  return useQueries({
    queries: ids.map(id => ({ ...sessionOwnerOptions(id), refetchOnWindowFocus: false })),
    combine: results => ids.filter((_, index) => results[index]?.error instanceof SessionNotFoundError),
  });
}
