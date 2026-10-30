import type { QueryClient } from "@tanstack/react-query";

import { buildSessionStreamUrl, fetchSessionTranscript } from "../adapters/session-api";
import { normalizeTranscriptEntries } from "../lib/message-schemas";
import { sessionKeys } from "../lib/query-keys";
import {
  invalidateSessionLiveQueries,
  invalidateSessionOwnLiveQueries,
  invalidateSessionUsageQueries,
} from "../lib/session-query-invalidation";
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
import {
  applySubagentsSnapshot,
  applySubagentUpdated,
  EMPTY_SUBAGENT_ROSTER,
  markSubagentRosterStale,
  type SubagentRoster,
} from "../lib/subagent-roster";
import type {
  NormalizedSessionTranscriptEntry,
  SessionConsumerDegradedPayload,
  SessionEventPayload,
  SessionPayload,
  SubagentsSnapshotPayload,
  SubagentUpdatedPayload,
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
  SessionLiveTailAppliedFrame,
  SessionLiveTailBatchResult,
  SessionLiveTailBatchStop,
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
  /**
   * Whether a burst of transcript deltas also rereads the session catalogs. A session shown
   * over its own board (#1052) leaves that to the catalog stream: rereading every column per
   * streamed burst would re-render the board under the drawer.
   */
  catalogRefreshOnDelta?: boolean;
}

function normalizeEntries(
  entries: TranscriptSnapshotPayload["entries"] | TranscriptDeltaPayload["entries"]
): Promise<NormalizedSessionTranscriptEntry[]> {
  return normalizeTranscriptEntries(entries);
}

interface NormalizedFrames {
  /** Normalized entries for the in-order prefix of frames that normalized. */
  entries: NormalizedSessionTranscriptEntry[][];
  failure: { error: unknown; index: number } | null;
}

/**
 * Normalizes every queued frame in one pass. A rejected batch is retried frame
 * by frame so the frames before the failing one still apply, as they would
 * have one at a time.
 */
async function normalizeFrames(frames: readonly SessionLiveTailFrame[]): Promise<NormalizedFrames> {
  try {
    // Normalization may leave out an entry whose every part was invalid, so each
    // entry carries its frame instead of relying on per-frame offsets.
    const flat = await normalizeTranscriptEntries(
      frames.flatMap((frame, frameIndex) =>
        frame.payload.entries.map(entry => ({ entry, frameIndex, message: entry.message }))
      )
    );
    const entries: NormalizedSessionTranscriptEntry[][] = frames.map(() => []);
    for (const item of flat) {
      entries[item.frameIndex]!.push({ ...item.entry, message: item.message });
    }
    return { entries, failure: null };
  } catch {
    const entries: NormalizedSessionTranscriptEntry[][] = [];
    for (const frame of frames) {
      try {
        entries.push(await normalizeEntries(frame.payload.entries));
      } catch (error) {
        return { entries, failure: { error, index: entries.length } };
      }
    }
    return { entries, failure: null };
  }
}

function reportInvalidationFailure(surface: string, error: unknown): void {
  console.error(`Failed to invalidate ${surface}`, error);
}

/**
 * The view had history loaded that this reset replaces: another generation
 * (rewind, clear), a sequence reset, or the same generation with the old
 * cursor retired by retention (`cursor_expired`) — all stated to the reader.
 */
function replacesLoadedHistory(data: SessionTranscriptData | undefined): boolean {
  const head = transcriptStreamCursor(data);
  return head.generation !== undefined && (head.afterSequence ?? 0) > 0;
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
  catalogRefreshOnDelta = true,
}: CreateSessionLiveTailRuntimeOptions): SessionLiveTailRuntime {
  const transcriptQueryKey = sessionKeys.transcript(workspaceId, sessionId);
  const readTranscript = () => queryClient.getQueryData<SessionTranscriptData>(transcriptQueryKey);
  const readCursor = () => transcriptStreamCursor(readTranscript()).afterSequence ?? 0;
  const subagentRosterKey = sessionKeys.subagentRoster(workspaceId, sessionId);
  const updateSubagentRoster = (update: (roster: SubagentRoster) => SubagentRoster) => {
    queryClient.setQueryData<SubagentRoster>(subagentRosterKey, existing =>
      update(existing ?? EMPTY_SUBAGENT_ROSTER)
    );
  };

  const invalidateSessionSurfaces = (reason?: "delta") => {
    const invalidation =
      reason === "delta" && !catalogRefreshOnDelta
        ? invalidateSessionOwnLiveQueries(queryClient, workspaceId, sessionId)
        : invalidateSessionLiveQueries(queryClient, workspaceId, sessionId);
    void invalidation.catch(error => reportInvalidationFailure("session live queries", error));
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

  const applyFrames = async (
    frames: readonly SessionLiveTailFrame[],
    signal: AbortSignal
  ): Promise<SessionLiveTailBatchResult> => {
    const [head] = frames;
    if (!head || !transcriptFrameCanApply(readTranscript(), head)) {
      return { applied: [], stop: "mismatch" };
    }

    const normalized = await normalizeFrames(frames);
    if (signal.aborted) return { applied: [], stop: "cancelled" };
    const applied: SessionLiveTailAppliedFrame[] = [];
    let stop: SessionLiveTailBatchStop | null = null;
    // One cache write for the whole batch; each frame still checks the fences
    // its predecessor left, so ordering and gap detection match per-frame writes.
    queryClient.setQueryData<SessionTranscriptData>(transcriptQueryKey, existing => {
      applied.length = 0;
      stop = null;
      let data = existing;
      for (const [index, entries] of normalized.entries.entries()) {
        const frame = frames[index]!;
        if (!transcriptFrameCanApply(data, frame)) {
          stop = "mismatch";
          break;
        }
        // A reset snapshot over loaded history of another generation is a stated
        // history rewrite (rewind, clear, retention), not a silent replacement.
        const result =
          frame.kind === "snapshot" && frame.payload.reset && replacesLoadedHistory(data)
            ? "reset"
            : "applied";
        data =
          frame.kind === "snapshot"
            ? applyTranscriptSnapshot(data, { ...frame.payload, entries }, frame.cursor)
            : applyTranscriptDelta(data, { ...frame.payload, entries }, frame.cursor);
        applied.push({ frame, result });
      }
      return data;
    });
    if (stop === null && normalized.failure) {
      stop = { error: normalized.failure.error, frame: frames[normalized.failure.index]! };
    }

    const resets = applied.filter(
      ({ frame }) => frame.kind === "snapshot" && frame.payload.reset
    ).length;
    if (resets > 0) {
      void queryClient.resetQueries({
        queryKey: sessionKeys.usage(workspaceId, sessionId),
        exact: true,
      });
      void queryClient.resetQueries({
        queryKey: sessionKeys.usageTurns(workspaceId, sessionId),
        exact: true,
      });
      queryClient.setQueryData<number>(
        sessionKeys.contextReset(workspaceId, sessionId),
        value => (value ?? 0) + resets
      );
    }
    return { applied, stop };
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
    const degradedListener: EventListener = event => {
      const payload = parseSessionStreamPayload<SessionConsumerDegradedPayload>(
        event as MessageEvent
      );
      // No SSE id on this event: it never advances the resume cursor.
      if (!payload || typeof payload.through_sequence !== "number") return;
      handlers.degraded(payload.through_sequence);
    };

    const subagentsSnapshotListener: EventListener = event => {
      const payload = parseSessionStreamPayload<SubagentsSnapshotPayload>(event as MessageEvent);
      if (!payload) return;
      updateSubagentRoster(() => applySubagentsSnapshot(payload));
    };
    const subagentUpdatedListener: EventListener = event => {
      const payload = parseSessionStreamPayload<SubagentUpdatedPayload>(event as MessageEvent);
      if (!payload?.subagent) return;
      updateSubagentRoster(roster => applySubagentUpdated(roster, payload));
    };

    let usageTimer: ReturnType<typeof setTimeout> | undefined;
    const flushUsageChanges = () => {
      usageTimer = undefined;
      void invalidateSessionUsageQueries(queryClient, workspaceId, sessionId).catch(error =>
        reportInvalidationFailure("session usage", error)
      );
    };
    const usageChangedListener: EventListener = () => {
      if (usageTimer !== undefined) clearTimeout(usageTimer);
      usageTimer = setTimeout(flushUsageChanges, 250);
    };
    let detach: () => void;
    try {
      // The moment the stream drops, held roster rows are unconfirmed (no tick,
      // no pulse) until the next subscription's snapshot confirms them.
      const handleError = (event: Event) => {
        if (queryClient.getQueryData(subagentRosterKey))
          updateSubagentRoster(markSubagentRosterStale);
        handlers.error(event);
      };
      detach = attachSessionStreamSource(source, handleError, {
        commandsChanged: commandsChangedListener,
        usageChanged: usageChangedListener,
        degraded: degradedListener,
        delta: deltaListener,
        goalSnapshot: goalSnapshotListener,
        snapshot: snapshotListener,
        subagentsSnapshot: subagentsSnapshotListener,
        subagentUpdated: subagentUpdatedListener,
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
      if (usageTimer !== undefined) {
        clearTimeout(usageTimer);
        flushUsageChanges();
      }
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
    applyFrames,
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
    refreshTranscript,
  };
}
