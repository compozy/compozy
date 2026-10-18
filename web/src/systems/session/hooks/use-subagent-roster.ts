import { useQuery } from "@tanstack/react-query";

import { sessionSubagentRosterOptions } from "../lib/query-options";
import { EMPTY_SUBAGENT_ROSTER, type SubagentRoster } from "../lib/subagent-roster";

/** The parent's roster as the session stream last wrote it; empty until the snapshot arrives. */
export function useSubagentRoster(workspaceId: string, sessionId: string): SubagentRoster {
  const { data } = useQuery(sessionSubagentRosterOptions(workspaceId, sessionId));
  return data ?? EMPTY_SUBAGENT_ROSTER;
}

/**
 * One slice of the roster. The query's structural sharing keeps the slice's
 * identity while its content is unchanged, so a consumer re-renders only when
 * what it shows changed.
 */
export function useSubagentRosterSelect<T>(
  workspaceId: string,
  sessionId: string,
  select: (roster: SubagentRoster) => T
): T {
  const { data } = useQuery({ ...sessionSubagentRosterOptions(workspaceId, sessionId), select });
  return data ?? select(EMPTY_SUBAGENT_ROSTER);
}
