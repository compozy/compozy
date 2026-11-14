import { useQuery } from "@tanstack/react-query";

import type { SessionMCPResponse } from "../adapters/session-mcp-api";
import { sessionMCPOptions } from "../lib/session-mcp-query-options";

export interface SessionMCPModel {
  /** The latest answer, kept after a failed reread so the popover can mark it out of date. */
  data: SessionMCPResponse | null;
  loading: boolean;
  failed: boolean;
  retry: () => void;
}

/** The composer's session MCP read, made only while `enabled` (the popover is open). */
export function useSessionMCP(
  workspaceId: string,
  sessionId: string,
  profile: string | undefined,
  enabled: boolean
): SessionMCPModel {
  const options = sessionMCPOptions(workspaceId, sessionId, profile);
  const queryEnabled = options.enabled === true && enabled;
  const query = useQuery({ ...options, enabled: queryEnabled });
  return {
    data: query.data ?? null,
    loading: query.isPending && queryEnabled,
    failed: query.isError,
    retry: () => void query.refetch(),
  };
}
