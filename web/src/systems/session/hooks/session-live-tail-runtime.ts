import type { QueryClient } from "@tanstack/react-query";

import { buildSessionStreamUrl, fetchSessionTranscript } from "../adapters/session-api";
import { normalizeTranscriptMessages } from "../lib/message-schemas";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionLiveQueries } from "../lib/session-query-invalidation";
import {
  formatSessionDebugError,
  recordSessionDebugEvent,
  SESSION_DEBUG_EVENTS,
} from "../lib/session-observability";
import {
  appendSyntheticTranscriptMessage,
  applyTranscriptDelta,
  applyTranscriptSnapshot,
  reconcileTranscriptTail,
  transcriptFrameMatches,
  transcriptPageFromResponse,
  transcriptStreamCursor,
  type SessionTranscriptData,
} from "../lib/session-transcript-query";
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
import type {
  SessionLiveTailApplyResult,
  SessionLiveTailFrame,
  SessionLiveTailRuntime,
  SessionLiveTailStreamHandlers,
} from "./session-live-tail-store-contract";
import {
  attachSessionStreamSource,
  type SessionStreamEventSourceFactory,
} from "./session-stream-source";

interface CreateSessionLiveTailRuntimeOptions {
  eventSourceFactory: SessionStreamEventSourceFactory;
  queryClient: QueryClient;
  sessionId: string;
  workspaceId: string;
}

async function normalizeEntries(
  entries: TranscriptSnapshotPayload["entries"] | TranscriptDeltaPayload["entries"]
): Promise<NormalizedSessionTranscriptEntry[]> {
  const messages = await normalizeTranscriptMessages(entries.map(entry => entry.message));
  return entries.map((entry, index) => ({ ...entry, message: messages[index]! }));
}

function reportInvalidationFailure(surface: string, error: unknown): void {
  console.error(`Failed to invalidate ${surface}`, error);
}

function transcriptFrameCanApply(
  data: SessionTranscriptData | undefined,
  frame: SessionLiveTailFrame
): boolean {
  return frame.kind === "snapshot" && frame.payload.reset
    ? true
    : transcriptFrameMatches(data, frame.payload);
}

export function createSessionLiveTailRuntime({
  eventSourceFactory,
  queryClient,
  sessionId,
  workspaceId,
}: CreateSessionLiveTailRuntimeOptions): SessionLiveTailRuntime {
  const transcriptQueryKey = sessionKeys.transcript(workspaceId, sessionId);
  const readTranscript = () => queryClient.getQueryData<SessionTranscriptData>(transcriptQueryKey);
  const readCursor = () => transcriptStreamCursor(readTranscript()).afterSequence ?? 0;

  const invalidateSessionSurfaces = () => {
    void invalidateSessionLiveQueries(queryClient, workspaceId, sessionId).catch(error =>
      reportInvalidationFailure("session live queries", error)
    );
  };

  const invalidateExact = (queryKey: readonly unknown[], surface: string) => {
    void queryClient
      .invalidateQueries({ queryKey, exact: true })
      .catch(error => reportInvalidationFailure(surface, error));
  };

  const refreshTranscript = async (signal: AbortSignal): Promise<void> => {
    const response = await fetchSessionTranscript(workspaceId, sessionId, {}, signal);
    if (signal.aborted) return;
    const tail = transcriptPageFromResponse(response);
    queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing =>
      reconcileTranscriptTail(existing, tail)
    );
  };

  const applyFrame = async (
    frame: SessionLiveTailFrame,
    signal: AbortSignal
  ): Promise<SessionLiveTailApplyResult> => {
    if (!transcriptFrameCanApply(readTranscript(), frame)) return "mismatch";

    const entries = await normalizeEntries(frame.payload.entries);
    if (signal.aborted) return "cancelled";
    let applied = false;
    queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing => {
      if (!transcriptFrameCanApply(existing, frame)) return existing;
      applied = true;
      return frame.kind === "snapshot"
        ? applyTranscriptSnapshot(existing, { ...frame.payload, entries }, frame.cursor)
        : applyTranscriptDelta(existing, { ...frame.payload, entries }, frame.cursor);
    });
    return applied ? "applied" : "mismatch";
  };

  const applyTerminal = (payload: SessionEventPayload | null, sequence: number) => {
    if (!payload) return;
    queryClient.setQueryData<SessionPayload>(
      sessionKeys.detail(workspaceId, sessionId),
      existing =>
        existing
          ? {
              ...existing,
              failure: payload.failure ?? existing.failure,
              state: "stopped",
              stop_detail: payload.stop_detail ?? existing.stop_detail,
              stop_reason: payload.stop_reason ?? existing.stop_reason,
              updated_at: payload.timestamp || existing.updated_at,
            }
          : existing
    );
    void queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(sessionId) });
    const failureMessage = terminalFailureMessage(payload, sessionId);
    if (failureMessage) {
      queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing =>
        appendSyntheticTranscriptMessage(existing, failureMessage, sequence)
      );
    }
  };

  const openStream = (handlers: SessionLiveTailStreamHandlers) => {
    const streamCursor = transcriptStreamCursor(readTranscript());
    const source = eventSourceFactory(buildSessionStreamUrl(workspaceId, sessionId, streamCursor));
    recordSessionDebugEvent(SESSION_DEBUG_EVENTS.sseOpen, {
      cursor: streamCursor.afterSequence ?? 0,
      session_id: sessionId,
      workspace_id: workspaceId,
    });

    const snapshotListener: EventListener = event => {
      const message = event as MessageEvent;
      const payload = parseSessionStreamPayload<TranscriptSnapshotPayload>(message);
      if (!payload) return;
      handlers.frame({
        containsClarification: entriesContainClarifyEvent(payload.entries),
        cursor: Math.max(payload.max_sequence, numberFromEventID(message.lastEventId) ?? 0),
        kind: "snapshot",
        payload,
      });
    };
    const deltaListener: EventListener = event => {
      const message = event as MessageEvent;
      const payload = parseSessionStreamPayload<TranscriptDeltaPayload>(message);
      if (!payload) return;
      handlers.frame({
        containsClarification: entriesContainClarifyEvent(payload.entries),
        cursor: Math.max(payload.cursor, numberFromEventID(message.lastEventId) ?? 0),
        kind: "delta",
        payload,
      });
    };
    const terminalListener: EventListener = event => {
      const message = event as MessageEvent;
      const payload = parseSessionStreamPayload<SessionEventPayload>(message);
      const sequence = Math.max(
        payload?.sequence ?? 0,
        numberFromEventID(message.lastEventId) ?? 0
      );
      handlers.terminal(payload, sequence);
    };
    const goalSnapshotListener: EventListener = () => handlers.goalChanged();
    const commandsChangedListener: EventListener = () => handlers.commandsChanged();

    let detach: () => void;
    try {
      detach = attachSessionStreamSource(source, handlers.error, {
        commandsChanged: commandsChangedListener,
        delta: deltaListener,
        goalSnapshot: goalSnapshotListener,
        snapshot: snapshotListener,
        terminal: terminalListener,
      });
    } catch (error) {
      try {
        source.close();
      } catch (closeError) {
        console.error("Failed to close rejected session stream", closeError);
      }
      throw error;
    }

    return (reason: string) => {
      detach();
      source.onmessage = null;
      source.onerror = null;
      try {
        source.close();
      } catch (error) {
        console.error("Failed to close session stream", error);
      }
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.sseClose, {
        cursor: readCursor(),
        reason,
        session_id: sessionId,
        workspace_id: workspaceId,
      });
    };
  };

  return {
    applyFrame,
    applyTerminal,
    invalidateClarifications: () =>
      invalidateExact(sessionKeys.clarifications(workspaceId, sessionId), "clarifications"),
    invalidateCommands: () =>
      invalidateExact(sessionKeys.commands(workspaceId, sessionId), "session commands"),
    invalidateGoal: () => invalidateExact(sessionKeys.goal(workspaceId, sessionId), "session Goal"),
    invalidateSessionSurfaces,
    openStream,
    recordApplyFailure: (frame, error) => {
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptApplyFailed, {
        cursor: readCursor(),
        error: formatSessionDebugError(error),
        frame: frame.kind,
        sequence: frame.cursor,
        session_id: sessionId,
        workspace_id: workspaceId,
      });
    },
    recordReconnect: (attempt, delayMs, error) => {
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.sseReconnect, {
        attempt,
        cursor: readCursor(),
        delay_ms: delayMs,
        ...(error === undefined ? {} : { error: formatSessionDebugError(error) }),
        session_id: sessionId,
        workspace_id: workspaceId,
      });
    },
    recordTranscriptFailure: (error, options) => {
      recordSessionDebugEvent(SESSION_DEBUG_EVENTS.transcriptFetchFailed, {
        cursor: readCursor(),
        error: formatSessionDebugError(error),
        ...(options.recovery ? { recovery: true } : {}),
        ...(options.sessionState === undefined ? {} : { session_state: options.sessionState }),
        session_id: sessionId,
        workspace_id: workspaceId,
      });
    },
    refreshTerminalTranscript: async () => {
      await queryClient.cancelQueries({ queryKey: transcriptQueryKey, exact: true });
      await queryClient.refetchQueries({
        queryKey: transcriptQueryKey,
        exact: true,
        type: "active",
      });
    },
    refreshTranscript,
  };
}
