import { useEffect } from "react";
import { useStore } from "@xstate/store-react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";

import { flattenTranscriptMessages } from "../lib/session-transcript-query";
import type { SessionTranscriptThreadStatus } from "../lib/session-transcript-thread-context-value";
import {
  isLiveSessionState,
  sessionDetailOptions,
  sessionTranscriptOptions,
} from "../lib/query-options";
import { toReadonlyThreadMessages } from "../lib/session-thread-repository";
import { createSessionLiveTailRuntime } from "./session-live-tail-runtime";
import { sessionLiveTailLogic } from "./session-live-tail-store";
import {
  defaultEventSourceFactory,
  type SessionStreamEventSourceFactory,
} from "./session-stream-source";

interface UseSessionLiveTailOptions {
  workspaceId: string;
  sessionId: string;
  enabled?: boolean;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  resetGeneration?: number;
}

export function useSessionLiveTail({
  workspaceId,
  sessionId,
  enabled = true,
  eventSourceFactory,
  resetGeneration = 0,
}: UseSessionLiveTailOptions) {
  const queryClient = useQueryClient();
  const store = useStore(sessionLiveTailLogic);
  const queryEnabled = workspaceId.trim() !== "" && sessionId.trim() !== "";
  const sessionState = useQuery({
    ...sessionDetailOptions(workspaceId, sessionId),
    enabled: queryEnabled && enabled,
  }).data?.state;
  const transcriptQuery = useInfiniteQuery({
    ...sessionTranscriptOptions(workspaceId, sessionId),
    enabled: queryEnabled,
  });
  const streamShouldOpen =
    enabled && queryEnabled && (sessionState == null || isLiveSessionState(sessionState));
  const canOpenStream =
    streamShouldOpen &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");

  useEffect(() => {
    const runtime = createSessionLiveTailRuntime({
      eventSourceFactory: eventSourceFactory ?? defaultEventSourceFactory,
      queryClient,
      sessionId,
      workspaceId,
    });
    store.trigger.configured({ enabled: canOpenStream, runtime });
    return () => store.trigger.disposed();
  }, [
    canOpenStream,
    eventSourceFactory,
    queryClient,
    resetGeneration,
    sessionId,
    store,
    workspaceId,
  ]);

  useEffect(() => {
    store.trigger.transcriptObserved({
      error: transcriptQuery.isError ? transcriptQuery.error : null,
      sessionState: sessionState ?? "unknown",
    });
  }, [sessionState, store, transcriptQuery.error, transcriptQuery.isError]);

  const transcriptMessages = flattenTranscriptMessages(transcriptQuery.data);
  const transcriptStatus: SessionTranscriptThreadStatus = transcriptQuery.isPending
    ? "pending"
    : transcriptQuery.isError
      ? "error"
      : "success";

  return {
    messages: toReadonlyThreadMessages(transcriptMessages),
    status: transcriptStatus,
    isPending: transcriptQuery.isPending,
    isError: transcriptQuery.isError,
    error: transcriptQuery.error,
    hasOlder: transcriptQuery.hasNextPage,
    isFetchingOlder: transcriptQuery.isFetchingNextPage,
    loadOlder: () => {
      void transcriptQuery.fetchNextPage();
    },
    retry: () => store.trigger.manualRecoveryRequested(),
  };
}

export type {
  SessionStreamEventSource,
  SessionStreamEventSourceFactory,
} from "./session-stream-source";
