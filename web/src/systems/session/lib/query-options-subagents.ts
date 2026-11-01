import { queryOptions } from "@tanstack/react-query";

import { fetchSessionSubagents, type SubagentPayload } from "../adapters/subagent-api";
import { sessionKeys } from "./query-keys";
import { EMPTY_SUBAGENT_ROSTER } from "./subagent-roster";

/**
 * The parent's subagent roster is stream-owned: the session stream's snapshot
 * and row updates write it, and nothing fetches it.
 */
export function sessionSubagentRosterOptions(workspaceId: string, sessionId: string) {
  return queryOptions({
    queryKey: sessionKeys.subagentRoster(workspaceId, sessionId),
    queryFn: () => EMPTY_SUBAGENT_ROSTER,
    enabled: false,
  });
}

/**
 * Every subagent of the parent for the mid-prompt control poll (newest first). The
 * poll replaces the whole roster, so it follows the cursor: a row left out would
 * read as an unconfirmed, still-running card.
 */
export function sessionSubagentRosterPollOptions(workspaceId: string, sessionId: string) {
  return queryOptions({
    queryKey: sessionKeys.subagentRosterPoll(workspaceId, sessionId),
    queryFn: async ({ signal }) => {
      const subagents: SubagentPayload[] = [];
      let cursor: string | undefined;
      do {
        const page = await fetchSessionSubagents(
          workspaceId,
          sessionId,
          cursor ? { limit: 200, cursor } : { limit: 200 },
          signal
        );
        subagents.push(...page.subagents);
        cursor = page.next_cursor ?? undefined;
      } while (cursor);
      return { subagents, next_cursor: null };
    },
    retry: false,
  });
}
