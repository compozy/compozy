import { useQuery } from "@tanstack/react-query";

import { sessionSubagentRosterOptions } from "../lib/query-options";
import { EMPTY_SUBAGENT_ROSTER, type SubagentRoster } from "../lib/subagent-roster";

/** The parent's roster as the session stream last wrote it; empty until the snapshot arrives. */
export function useSubagentRoster(workspaceId: string, sessionId: string): SubagentRoster {
  const { data } = useQuery(sessionSubagentRosterOptions(workspaceId, sessionId));
  return data ?? EMPTY_SUBAGENT_ROSTER;
}
