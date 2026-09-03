"use client";

import { hashKey, useQueryClient } from "@tanstack/react-query";
import { useSyncExternalStore } from "react";

import { useSecondClock } from "@/hooks/use-second-clock";
import type { TerminalRecordingState } from "../components/terminal-header";
import { terminalKeys } from "../lib/query-keys";
import { formatRecordingElapsed, type TerminalRecordingMap } from "../lib/terminal-recording-state";
import type { TerminalScopeKey } from "../types";

const EMPTY_RECORDINGS: TerminalRecordingMap = {};

function subscribeIdle(): () => void {
  return () => undefined;
}

function getEmptyRecordings(): TerminalRecordingMap {
  return EMPTY_RECORDINGS;
}

/**
 * The host's live recording map: cache truth in, elapsed chips out.
 *
 * The catalog stream writes this cache. There is no GET for in-progress
 * recordings, so the hook reads the query cache directly. Reconnect presence
 * is that stream's snapshot/open clear plus the following started events. One
 * The process-wide second clock runs while any recording consumer is active.
 */
export function useTerminalRecordings(
  scope: TerminalScopeKey,
  enabled: boolean
): Readonly<Record<string, TerminalRecordingState>> {
  const queryClient = useQueryClient();
  const workspaceId = scope.workspaceId;
  const profileKey = scope.profileKey;
  const canRead = enabled && workspaceId !== "";
  function subscribe(onStoreChange: () => void) {
    const queryKey = terminalKeys.recordings({ workspaceId, profileKey });
    const hash = hashKey(queryKey);
    return queryClient.getQueryCache().subscribe(event => {
      if (event.query.queryHash === hash) onStoreChange();
    });
  }
  function getSnapshot() {
    if (!canRead) return EMPTY_RECORDINGS;
    return (
      queryClient.getQueryData<TerminalRecordingMap>(
        terminalKeys.recordings({ workspaceId, profileKey })
      ) ?? EMPTY_RECORDINGS
    );
  }
  const map = useSyncExternalStore(
    canRead ? subscribe : subscribeIdle,
    getSnapshot,
    getEmptyRecordings
  );
  const hasAny = Object.keys(map).length > 0;
  const now = useSecondClock(canRead && hasAny);

  const recordings: Record<string, TerminalRecordingState> = {};
  for (const [terminalId, entry] of Object.entries(map)) {
    const elapsed = formatRecordingElapsed(entry.at, now);
    if (elapsed !== null) recordings[terminalId] = { elapsed };
  }
  return recordings;
}
