import { useEffect, useRef } from "react";
import {
  type QueryClient,
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import { buildSessionStreamUrl, fetchSessionTranscript } from "../adapters/session-api";
import { normalizeTranscriptMessages } from "../lib/message-schemas";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionLiveQueries } from "../lib/session-query-invalidation";
import {
  isLiveSessionState,
  sessionDetailOptions,
  sessionTranscriptOptions,
} from "../lib/query-options";
import { toReadonlyThreadMessages } from "../lib/session-thread-repository";
import {
  formatSessionDebugError,
  recordSessionDebugEvent,
  SESSION_DEBUG_EVENTS,
} from "../lib/session-observability";
import {
  appendSyntheticTranscriptMessage,
  applyTranscriptDelta,
  applyTranscriptSnapshot,
  flattenTranscriptMessages,
  reconcileTranscriptTail,
  transcriptPageFromResponse,
  transcriptFrameMatches,
  transcriptStreamCursor,
  type SessionTranscriptData,
} from "../lib/session-transcript-query";
import type { SessionTranscriptThreadStatus } from "../lib/session-transcript-thread-context-value";
import type {
  NormalizedSessionTranscriptEntry,
  SessionEventPayload,
  SessionPayload,
  TranscriptDeltaPayload,
  TranscriptSnapshotPayload,
} from "../types";
import {
  entriesContainClarifyEvent,
  numberFromEventID,
  parseSessionStreamPayload,
  terminalFailureMessage,
} from "./session-live-tail-helpers";

interface SessionStreamEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener?: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

type SessionStreamEventSourceFactory = (url: string) => SessionStreamEventSource;

interface UseSessionLiveTailOptions {
  workspaceId: string;
  sessionId: string;
  eventSourceFactory?: SessionStreamEventSourceFactory;
}

const TRANSCRIPT_SNAPSHOT_EVENT = "transcript_snapshot";
const TRANSCRIPT_DELTA_EVENT = "transcript_delta";
const GOAL_SNAPSHOT_CHANGED_EVENT = "goal_snapshot_changed";
const SESSION_STOPPED_EVENT = "session_stopped";
const SESSION_DONE_EVENT = "done";
const STREAM_ERROR_EVENT = "error";
const RECONNECT_BASE_DELAY_MS = 250;
const RECONNECT_MAX_DELAY_MS = 4_000;
const TRANSCRIPT_ERROR_RECOVERY_DELAY_MS = 5_000;
type TranscriptApplyFrame = "snapshot" | "delta";

function defaultEventSourceFactory(url: string): SessionStreamEventSource {
  return new EventSource(url);
}

interface SessionStreamListeners {
  delta: EventListener;
  error: EventListener;
  goalSnapshot: EventListener;
  snapshot: EventListener;
  terminal: EventListener;
}

function attachSessionStreamSource(
  source: SessionStreamEventSource,
  handleError: (event: Event) => void,
  listeners: SessionStreamListeners
): () => void {
  source.onmessage = null;
  source.onerror = handleError;
  source.addEventListener(TRANSCRIPT_SNAPSHOT_EVENT, listeners.snapshot);
  source.addEventListener(TRANSCRIPT_DELTA_EVENT, listeners.delta);
  source.addEventListener(GOAL_SNAPSHOT_CHANGED_EVENT, listeners.goalSnapshot);
  source.addEventListener(SESSION_STOPPED_EVENT, listeners.terminal);
  source.addEventListener(SESSION_DONE_EVENT, listeners.terminal);
  source.addEventListener(STREAM_ERROR_EVENT, listeners.error);
  return () => {
    if (!source.removeEventListener) return;
    source.removeEventListener(TRANSCRIPT_SNAPSHOT_EVENT, listeners.snapshot);
    source.removeEventListener(TRANSCRIPT_DELTA_EVENT, listeners.delta);
    source.removeEventListener(GOAL_SNAPSHOT_CHANGED_EVENT, listeners.goalSnapshot);
    source.removeEventListener(SESSION_STOPPED_EVENT, listeners.terminal);
    source.removeEventListener(SESSION_DONE_EVENT, listeners.terminal);
    source.removeEventListener(STREAM_ERROR_EVENT, listeners.error);
  };
}

async function normalizeEntries(
  entries: TranscriptSnapshotPayload["entries"] | TranscriptDeltaPayload["entries"]
): Promise<NormalizedSessionTranscriptEntry[]> {
  const messages = await normalizeTranscriptMessages(entries.map(entry => entry.message));
  return entries.map((entry, index) => ({ ...entry, message: messages[index]! }));
}

async function refreshTranscriptTail(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  const response = await fetchSessionTranscript(workspaceId, sessionId);
  const queryKey = sessionKeys.transcript(workspaceId, sessionId);
  const tail = transcriptPageFromResponse(response);
  queryClient.setQueryData<SessionTranscriptData>(queryKey, existing =>
    reconcileTranscriptTail(existing, tail)
  );
}

export function useSessionLiveTail({
  workspaceId,
  sessionId,
  eventSourceFactory,
}: UseSessionLiveTailOptions) {
  const queryClient = useQueryClient();
  const reloadTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastTranscriptErrorRef = useRef<unknown>(null);
  const sourceFactory = eventSourceFactory ?? defaultEventSourceFactory;
  const hasCustomFactory = Boolean(eventSourceFactory);
  const sessionState = useQuery(sessionDetailOptions(workspaceId, sessionId)).data?.state;
  const streamShouldOpen = sessionState == null || isLiveSessionState(sessionState);
  const transcriptQuery = useInfiniteQuery(sessionTranscriptOptions(workspaceId, sessionId));
  const transcriptMessages = flattenTranscriptMessages(transcriptQuery.data);
  const transcriptStatus: SessionTranscriptThreadStatus = transcriptQuery.isPending
    ? "pending"
    : transcriptQuery.isError
      ? "error"
      : "success";
  const readonlyMessages = toReadonlyThreadMessages(transcriptMessages);

  useEffect(() => {
    return () => {
      if (reloadTimerRef.current) clearTimeout(reloadTimerRef.current);
    };
  }, []);

  useEffect(() => {
    if (!transcriptQuery.isError) {
      lastTranscriptErrorRef.current = null;
      return;
    }
    if (lastTranscriptErrorRef.current === transcriptQuery.error) return;
    lastTranscriptErrorRef.current = transcriptQuery.error;
    const data = queryClient.getQueryData<SessionTranscriptData>(
      sessionKeys.transcript(workspaceId, sessionId)
    );
    recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptFetchFailed, {
      cursor: transcriptStreamCursor(data).afterSequence ?? 0,
      error: formatSessionDebugError(transcriptQuery.error),
      session_id: sessionId,
      session_state: sessionState ?? "unknown",
      workspace_id: workspaceId,
    });
  }, [
    queryClient,
    sessionId,
    sessionState,
    transcriptQuery.error,
    transcriptQuery.isError,
    workspaceId,
  ]);

  useEffect(() => {
    if (!transcriptQuery.isError || !streamShouldOpen) return undefined;
    let disposed = false;
    let recoveryTimer: ReturnType<typeof setTimeout> | null = null;

    const recover = async () => {
      try {
        await refreshTranscriptTail(queryClient, workspaceId, sessionId);
      } catch (error) {
        if (disposed) return;
        recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptFetchFailed, {
          cursor:
            transcriptStreamCursor(
              queryClient.getQueryData<SessionTranscriptData>(
                sessionKeys.transcript(workspaceId, sessionId)
              )
            ).afterSequence ?? 0,
          error: formatSessionDebugError(error),
          recovery: true,
          session_id: sessionId,
          workspace_id: workspaceId,
        });
        recoveryTimer = setTimeout(() => {
          void recover();
        }, TRANSCRIPT_ERROR_RECOVERY_DELAY_MS);
      }
    };

    recoveryTimer = setTimeout(() => {
      void recover();
    }, TRANSCRIPT_ERROR_RECOVERY_DELAY_MS);
    return () => {
      disposed = true;
      if (recoveryTimer) clearTimeout(recoveryTimer);
    };
  }, [queryClient, sessionId, streamShouldOpen, transcriptQuery.isError, workspaceId]);

  useEffect(() => {
    if (
      workspaceId.trim() === "" ||
      sessionId.trim() === "" ||
      !streamShouldOpen ||
      typeof window === "undefined" ||
      (!hasCustomFactory && typeof EventSource === "undefined")
    ) {
      return undefined;
    }

    const transcriptQueryKey = sessionKeys.transcript(workspaceId, sessionId);
    const readTranscript = () =>
      queryClient.getQueryData<SessionTranscriptData>(transcriptQueryKey);
    const readCursor = () => transcriptStreamCursor(readTranscript()).afterSequence ?? 0;
    const invalidateSessionSurfaces = () => {
      void invalidateSessionLiveQueries(queryClient, workspaceId, sessionId);
    };
    const invalidateClarifications = () =>
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.clarifications(workspaceId, sessionId),
        exact: true,
      });
    const scheduleSurfaceRefresh = () => {
      if (reloadTimerRef.current) clearTimeout(reloadTimerRef.current);
      reloadTimerRef.current = setTimeout(() => {
        reloadTimerRef.current = null;
        invalidateSessionSurfaces();
      }, 120);
    };

    let disposed = false;
    let reconnectAttempt = 0;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let source: SessionStreamEventSource | null = null;
    let detachSourceListeners: (() => void) | null = null;
    let transcriptApplyQueue = Promise.resolve();

    const clearReconnectTimer = () => {
      if (!reconnectTimer) return;
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    };
    const closeCurrentSource = (reason: string) => {
      detachSourceListeners?.();
      detachSourceListeners = null;
      if (!source) return;
      const cursor = readCursor();
      source.onmessage = null;
      source.onerror = null;
      source.close();
      source = null;
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.sseClose, {
        cursor,
        reason,
        session_id: sessionId,
        workspace_id: workspaceId,
      });
    };
    const scheduleReconnect = () => {
      if (disposed) return;
      clearReconnectTimer();
      const cursor = readCursor();
      const delay = Math.min(
        RECONNECT_BASE_DELAY_MS * 2 ** reconnectAttempt,
        RECONNECT_MAX_DELAY_MS
      );
      reconnectAttempt += 1;
      closeCurrentSource("reconnect");
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.sseReconnect, {
        attempt: reconnectAttempt,
        cursor,
        delay_ms: delay,
        session_id: sessionId,
        workspace_id: workspaceId,
      });
      reconnectTimer = setTimeout(openSource, delay);
    };
    const recordApplyFailure = (frame: TranscriptApplyFrame, sequence: number, error: unknown) => {
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptApplyFailed, {
        cursor: readCursor(),
        error: formatSessionDebugError(error),
        frame,
        sequence,
        session_id: sessionId,
        workspace_id: workspaceId,
      });
      void refreshTranscriptTail(queryClient, workspaceId, sessionId).catch(recoveryError => {
        recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptFetchFailed, {
          cursor: readCursor(),
          error: formatSessionDebugError(recoveryError),
          recovery: true,
          session_id: sessionId,
          workspace_id: workspaceId,
        });
      });
    };
    const enqueueApply = (
      frame: TranscriptApplyFrame,
      sequence: number,
      apply: () => Promise<void>
    ) => {
      transcriptApplyQueue = transcriptApplyQueue.then(async () => {
        if (disposed) return;
        try {
          await apply();
        } catch (error) {
          recordApplyFailure(frame, sequence, error);
        }
      });
    };

    const applySnapshot = (event: MessageEvent) => {
      const payload = parseSessionStreamPayload<TranscriptSnapshotPayload>(event);
      if (!payload) return;
      reconnectAttempt = 0;
      const eventCursor = Math.max(payload.max_sequence, numberFromEventID(event.lastEventId) ?? 0);
      if (!payload.reset && !transcriptFrameMatches(readTranscript(), payload)) {
        scheduleReconnect();
        return;
      }
      enqueueApply("snapshot", eventCursor, async () => {
        const entries = await normalizeEntries(payload.entries);
        queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing => {
          if (!payload.reset && !transcriptFrameMatches(existing, payload)) return existing;
          return applyTranscriptSnapshot(existing, { ...payload, entries }, eventCursor);
        });
      });
      invalidateSessionSurfaces();
      if (entriesContainClarifyEvent(payload.entries)) invalidateClarifications();
    };

    const applyDelta = (event: MessageEvent) => {
      const payload = parseSessionStreamPayload<TranscriptDeltaPayload>(event);
      if (!payload) return;
      reconnectAttempt = 0;
      const eventCursor = Math.max(payload.cursor, numberFromEventID(event.lastEventId) ?? 0);
      if (!transcriptFrameMatches(readTranscript(), payload)) {
        scheduleReconnect();
        return;
      }
      enqueueApply("delta", eventCursor, async () => {
        const entries = await normalizeEntries(payload.entries);
        queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing => {
          if (!transcriptFrameMatches(existing, payload)) return existing;
          return applyTranscriptDelta(existing, { ...payload, entries }, eventCursor);
        });
      });
      scheduleSurfaceRefresh();
      if (entriesContainClarifyEvent(payload.entries)) invalidateClarifications();
    };

    const handleTerminalEvent = (event: MessageEvent) => {
      const payload = parseSessionStreamPayload<SessionEventPayload>(event);
      const sequence = Math.max(payload?.sequence ?? 0, numberFromEventID(event.lastEventId) ?? 0);
      if (payload) {
        queryClient.setQueryData<SessionPayload>(
          sessionKeys.detail(workspaceId, sessionId),
          existing =>
            existing
              ? {
                  ...existing,
                  state: "stopped",
                  stop_reason: payload.stop_reason ?? existing.stop_reason,
                  stop_detail: payload.stop_detail ?? existing.stop_detail,
                  failure: payload.failure ?? existing.failure,
                  updated_at: payload.timestamp || existing.updated_at,
                }
              : existing
        );
        const failureMessage = terminalFailureMessage(payload, sessionId);
        if (failureMessage) {
          queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing =>
            appendSyntheticTranscriptMessage(existing, failureMessage, sequence)
          );
        }
      }
      clearReconnectTimer();
      closeCurrentSource("terminal");
      invalidateSessionSurfaces();
    };
    const handleGoalSnapshotChanged = () => {
      reconnectAttempt = 0;
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.goal(workspaceId, sessionId),
        exact: true,
      });
    };
    const handleError = () => {
      scheduleSurfaceRefresh();
      scheduleReconnect();
    };

    const snapshotListener = applySnapshot as EventListener;
    const deltaListener = applyDelta as EventListener;
    const goalSnapshotListener = handleGoalSnapshotChanged as EventListener;
    const terminalListener = handleTerminalEvent as EventListener;
    const streamErrorListener = handleError as EventListener;

    function openSource() {
      if (disposed) return;
      const streamCursor = transcriptStreamCursor(readTranscript());
      const nextSource = sourceFactory(buildSessionStreamUrl(workspaceId, sessionId, streamCursor));
      source = nextSource;
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.sseOpen, {
        cursor: streamCursor.afterSequence ?? 0,
        session_id: sessionId,
        workspace_id: workspaceId,
      });
      detachSourceListeners = attachSessionStreamSource(nextSource, handleError, {
        delta: deltaListener,
        error: streamErrorListener,
        goalSnapshot: goalSnapshotListener,
        snapshot: snapshotListener,
        terminal: terminalListener,
      });
    }

    openSource();
    return () => {
      disposed = true;
      clearReconnectTimer();
      closeCurrentSource("cleanup");
    };
  }, [
    eventSourceFactory,
    hasCustomFactory,
    queryClient,
    sessionId,
    sourceFactory,
    streamShouldOpen,
    workspaceId,
  ]);

  return {
    messages: readonlyMessages,
    status: transcriptStatus,
    isPending: transcriptQuery.isPending,
    isError: transcriptQuery.isError,
    error: transcriptQuery.error,
    hasOlder: transcriptQuery.hasNextPage,
    isFetchingOlder: transcriptQuery.isFetchingNextPage,
    loadOlder: () => {
      void transcriptQuery.fetchNextPage();
    },
    retry: () => {
      void refreshTranscriptTail(queryClient, workspaceId, sessionId).catch(error => {
        recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptFetchFailed, {
          cursor:
            transcriptStreamCursor(
              queryClient.getQueryData<SessionTranscriptData>(
                sessionKeys.transcript(workspaceId, sessionId)
              )
            ).afterSequence ?? 0,
          error: formatSessionDebugError(error),
          recovery: true,
          session_id: sessionId,
          workspace_id: workspaceId,
        });
      });
    },
  };
}

export type { SessionStreamEventSource, SessionStreamEventSourceFactory };
