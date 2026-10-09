import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { sessionKeys } from "../lib/query-keys";
import { sessionSubagentRosterPollOptions } from "../lib/query-options";
import { applySubagentsSnapshot, type SubagentRoster } from "../lib/subagent-roster";

/**
 * While a prompt POST streams, the session SSE that feeds the roster stays
 * closed (`use-session-runtime-extensions`), so the bounded control poll also
 * reads the parent's subagents and writes them into the roster as a snapshot:
 * cards, the banner and "N agents running" stay current mid-turn. The live
 * tail's own snapshot replaces it when the stream reopens on settle.
 */
export function useSubagentRosterControlPoll({
  workspaceId,
  sessionId,
  enabled,
  intervalMs,
}: {
  workspaceId: string;
  sessionId: string;
  enabled: boolean;
  intervalMs: number;
}): void {
  const queryClient = useQueryClient();
  const poll = useQuery({
    ...sessionSubagentRosterPollOptions(workspaceId, sessionId),
    enabled: enabled && workspaceId !== "" && sessionId !== "",
    refetchInterval: enabled ? intervalMs : false,
  });
  const page = enabled ? poll.data : undefined;
  useEffect(() => {
    if (!page) return;
    queryClient.setQueryData<SubagentRoster>(
      sessionKeys.subagentRoster(workspaceId, sessionId),
      applySubagentsSnapshot({ session_id: sessionId, subagents: page.subagents })
    );
  }, [page, queryClient, sessionId, workspaceId]);
}
