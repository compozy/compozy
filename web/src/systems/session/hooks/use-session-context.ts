import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import {
  sessionContextResetOptions,
  sessionUsageOptions,
  sessionUsageTurnsOptions,
} from "../lib/query-options";
import { deriveSessionContext, retainSessionUsage } from "../lib/session-context";
import type { SessionState, SessionUsagePayload } from "../types";

/** A window owns the retention lifetime. Transcript content never supplies usage values. */
export function useSessionContext(
  sessionId: string,
  workspaceId: string,
  sessionState?: SessionState,
  options: { enabled?: boolean } = {}
) {
  const query = useQuery({
    ...sessionUsageOptions(workspaceId, sessionId, sessionState),
    enabled: (options.enabled ?? true) && !!sessionId && !!workspaceId,
  });
  const reset = useQuery(sessionContextResetOptions(workspaceId, sessionId));
  const identity = `${workspaceId}/${sessionId}/${reset.data ?? 0}`;
  const [retained, setRetained] = useState<{
    identity: string;
    source?: SessionUsagePayload;
    usage?: SessionUsagePayload;
  }>({ identity });
  let current = retained;
  if (retained.identity !== identity || retained.source !== query.data) {
    current = {
      identity,
      source: query.data,
      usage: retainSessionUsage(
        retained.identity === identity ? retained.usage : undefined,
        query.data
      ),
    };
    setRetained(current);
  }
  return {
    usage: current.usage,
    context: deriveSessionContext(current.usage?.context, {
      unavailable: query.isError || query.data?.context.state === "unavailable",
      loading: query.isLoading,
      stopped: sessionState === "stopped",
    }),
  };
}

export function useSessionUsageTurns(
  sessionId: string,
  workspaceId: string,
  sessionState?: SessionState,
  options: { enabled?: boolean } = {}
) {
  return useQuery({
    ...sessionUsageTurnsOptions(workspaceId, sessionId, sessionState),
    enabled: (options.enabled ?? true) && !!sessionId && !!workspaceId,
  });
}
