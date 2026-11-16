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

export interface SessionMCPReadGate {
  /** The popover is open: it lists the session's servers. */
  open: boolean;
  /** The session payload's summary has a change waiting for the next message. */
  pendingReload: boolean;
}

/**
 * The composer's session MCP read. It runs while the popover is open, and while the closed trigger
 * has a pending change: a failed reload keeps its change pending, so that is the only closed state
 * whose reload outcome the trigger must show. The stream's MCP events reread it while it runs.
 */
export function useSessionMCP(
  workspaceId: string,
  sessionId: string,
  profile: string | undefined,
  gate: SessionMCPReadGate
): SessionMCPModel {
  const options = sessionMCPOptions(workspaceId, sessionId, profile);
  const queryEnabled = options.enabled === true && (gate.open || gate.pendingReload);
  const query = useQuery({ ...options, enabled: queryEnabled });
  return {
    data: query.data ?? null,
    loading: query.isPending && queryEnabled,
    failed: query.isError,
    retry: () => void query.refetch(),
  };
}
