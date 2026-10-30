import { useQueries } from "@tanstack/react-query";
import { sessionScopedDetailOptions } from "../lib/query-options";
import type { SessionPayload } from "../types";

export function useCurrentDeleteSessions(
  sessions: readonly SessionPayload[] | undefined,
  openedAt: number
) {
  // Catalog rows may lag the detail stream; keep an open confirmation current.
  const deleteReads = useQueries({
    queries: (sessions ?? []).map(session => ({
      ...sessionScopedDetailOptions(
        session.id,
        { profile: session.profile_name },
        { liveTail: false }
      ),
      refetchOnMount: "always" as const,
    })),
  });
  return sessions?.map((session, index) => {
    const read = deleteReads[index];
    return read && read.dataUpdatedAt >= openedAt ? (read.data ?? session) : session;
  });
}
