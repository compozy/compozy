import { hashKey, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { sessionKeys } from "../lib/query-keys";
import {
  CHANGES_REFRESH_DEBOUNCE_MS,
  CHANGES_REFRESH_MAX_WAIT_MS,
  createDebouncedRefresh,
  settledToolCount,
} from "../lib/session-changes-loading";
import type { SessionTranscriptData } from "../lib/session-transcript-query";
import type { SessionPayload } from "../types";

function activeTurnId(session: SessionPayload | undefined): string {
  return session?.activity?.turn_id ?? "";
}

/**
 * Event-driven Changes refresh (US-010.AC-1). It watches what the session's live stream
 * already writes — the transcript cache and the session read — and debounces every settled
 * tool call or ended turn into one summary refetch. It opens no stream of its own.
 */
export function useSessionChangesLiveRefresh({
  workspaceId,
  sessionId,
  enabled,
  onRefresh,
}: {
  workspaceId: string;
  sessionId: string;
  enabled: boolean;
  onRefresh: () => void;
}): void {
  const queryClient = useQueryClient();
  const onRefreshRef = useRef(onRefresh);
  useEffect(() => {
    onRefreshRef.current = onRefresh;
  });

  useEffect(() => {
    if (!enabled || !workspaceId || !sessionId) return;
    const transcriptKey = sessionKeys.transcript(workspaceId, sessionId);
    const detailKey = sessionKeys.detail(workspaceId, sessionId);
    const transcriptHash = hashKey(transcriptKey);
    const detailHash = hashKey(detailKey);
    const scheduler = createDebouncedRefresh({
      delayMs: CHANGES_REFRESH_DEBOUNCE_MS,
      maxWaitMs: CHANGES_REFRESH_MAX_WAIT_MS,
      onRefresh: () => onRefreshRef.current(),
    });
    let tools = settledToolCount(queryClient.getQueryData<SessionTranscriptData>(transcriptKey));
    let turn = activeTurnId(queryClient.getQueryData<SessionPayload>(detailKey));
    const unsubscribe = queryClient.getQueryCache().subscribe(event => {
      if (event.type !== "updated" || event.action.type !== "success") return;
      if (event.query.queryHash === transcriptHash) {
        const next = settledToolCount(event.query.state.data as SessionTranscriptData | undefined);
        if (next !== tools) scheduler.signal();
        tools = next;
      } else if (event.query.queryHash === detailHash) {
        const next = activeTurnId(event.query.state.data as SessionPayload | undefined);
        // A turn ended: the agent may have written files outside any tool call.
        if (turn !== "" && next !== turn) scheduler.signal();
        turn = next;
      }
    });
    return () => {
      unsubscribe();
      scheduler.cancel();
    };
  }, [enabled, queryClient, sessionId, workspaceId]);
}
