import { useEffect, useEffectEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { createStreamEventSource } from "@/lib/ticketed-event-source";

import {
  appendExtensionLogEntry,
  buildExtensionLogsStreamUrl,
  extensionLogCursor,
  normalizeExtensionLogSnapshot,
  parseExtensionLogEvent,
  parseExtensionLogResetEvent,
} from "../lib/extension-log-stream";
import { extensionLogsOptions } from "../lib/query-options";
import type { ExtensionLogsSnapshot } from "../types";
import { extensionLogsLogic, type ExtensionLogStreamStatus } from "./extension-logs-store";

/** The daemon publishes one delta as this named SSE event (never `message`). */
export const EXTENSION_LOG_EVENT_NAME = "extension_log";
/** A recreated daemon ring publishes its complete replacement snapshot through this event. */
export const EXTENSION_LOG_RESET_EVENT_NAME = "extension_log_reset";

export type { ExtensionLogStreamStatus } from "./extension-logs-store";

export interface ExtensionLogEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener?: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
  onerror: ((event: Event) => void) | null;
  onopen: ((event: Event) => void) | null;
}

export interface UseExtensionLogsOptions {
  name: string;
  workspaceId?: string | null;
  enabled?: boolean;
  eventSourceFactory?: (url: string) => ExtensionLogEventSource;
}

export interface ExtensionLogsModel {
  entries: ExtensionLogsSnapshot["logs"];
  error: Error | null;
  follow: boolean;
  isLoading: boolean;
  refetch: () => void;
  setFollow: (follow: boolean) => void;
  status: ExtensionLogStreamStatus;
}

function defaultEventSourceFactory(url: string): ExtensionLogEventSource {
  return createStreamEventSource(url);
}

function extensionLogError(error: unknown, fallback: string): Error {
  return error instanceof Error ? error : new Error(fallback);
}

/**
 * Query owns the scoped log snapshot. The effect owns only its EventSource and serialized cache
 * writes: it fetches one fresh `(stream_epoch, sequence)` baseline before opening the stream, then
 * applies deltas or an atomic replacement snapshot when the daemon recreates its in-memory ring.
 */
export function useExtensionLogs({
  name,
  workspaceId,
  enabled = true,
  eventSourceFactory,
}: UseExtensionLogsOptions): ExtensionLogsModel {
  const queryClient = useQueryClient();
  const normalizedName = name.trim();
  const normalizedWorkspaceId = workspaceId?.trim() || undefined;
  const instance = `${normalizedWorkspaceId ?? ""}\u0000${normalizedName}`;
  const { store } = useStoreBinding(instance, () => extensionLogsLogic.createStore());
  const connectionError = useSelector(store, snapshot => snapshot.context.error);
  const follow = useSelector(store, snapshot => snapshot.context.follow);
  const retryToken = useSelector(store, snapshot => snapshot.context.retryToken);
  const streamStatus = useSelector(store, snapshot => snapshot.context.streamStatus);
  const canStream = eventSourceFactory !== undefined || typeof EventSource !== "undefined";
  const openEventSource = useEffectEvent((url: string) =>
    (eventSourceFactory ?? defaultEventSourceFactory)(url)
  );
  const historyOptions = extensionLogsOptions(normalizedName, {
    workspaceId: normalizedWorkspaceId,
  });
  const history = useQuery({
    ...historyOptions,
    enabled: enabled && normalizedName !== "" && !canStream,
  });

  const status: ExtensionLogStreamStatus = !follow
    ? "paused"
    : enabled && normalizedName !== ""
      ? streamStatus
      : "idle";

  useEffect(() => {
    if (!enabled || normalizedName === "" || !follow || !canStream) return undefined;

    const cacheKey = extensionLogsOptions(normalizedName, {
      workspaceId: normalizedWorkspaceId,
    }).queryKey;
    store.trigger.connecting();
    const generation = store.getSnapshot().context.generation;
    let closed = false;
    let source: ExtensionLogEventSource | undefined;
    let handleLog: EventListener | undefined;
    let handleReset: EventListener | undefined;
    let writeQueue = Promise.resolve();

    const generationIsCurrent = () =>
      !closed && store.getSnapshot().context.generation === generation;

    const queueSnapshotWrite = (
      update: (current: ExtensionLogsSnapshot | undefined) => ExtensionLogsSnapshot | undefined
    ) => {
      writeQueue = writeQueue.then(async () => {
        if (!generationIsCurrent()) return;
        try {
          await queryClient.cancelQueries({ exact: true, queryKey: cacheKey });
        } catch (error) {
          console.error("Failed to fence the extension log history read", error);
          return;
        }
        if (!generationIsCurrent()) return;
        queryClient.setQueryData<ExtensionLogsSnapshot>(cacheKey, update);
      });
    };

    const start = async () => {
      let baseline: ExtensionLogsSnapshot;
      try {
        await queryClient.cancelQueries({ exact: true, queryKey: cacheKey });
      } catch (error) {
        if (generationIsCurrent()) {
          store.trigger.streamFailed({
            error: extensionLogError(error, "Failed to fence the extension log history read"),
            generation,
          });
          console.error("Failed to fence the extension log history read", error);
        }
        return;
      }
      try {
        baseline = normalizeExtensionLogSnapshot(
          await queryClient.fetchQuery(
            extensionLogsOptions(normalizedName, { workspaceId: normalizedWorkspaceId })
          )
        );
      } catch (error) {
        const retained = queryClient.getQueryData<ExtensionLogsSnapshot>(cacheKey);
        if (!generationIsCurrent() || !retained) {
          if (generationIsCurrent()) {
            store.trigger.streamFailed({
              error: extensionLogError(error, "Failed to load the extension log baseline"),
              generation,
            });
            console.error("Failed to load the extension log baseline", error);
          }
          return;
        }
        baseline = normalizeExtensionLogSnapshot(retained);
        console.error("Failed to refresh the extension log baseline; using retained cursor", error);
      }
      if (!generationIsCurrent()) return;
      queryClient.setQueryData(cacheKey, baseline);
      try {
        source = openEventSource(
          buildExtensionLogsStreamUrl(normalizedName, {
            after: extensionLogCursor(baseline.logs),
            streamEpoch: baseline.stream_epoch,
            workspaceId: normalizedWorkspaceId,
          })
        );
      } catch (error) {
        if (generationIsCurrent()) {
          store.trigger.streamFailed({
            error: extensionLogError(error, "Failed to open the extension log stream"),
            generation,
          });
          console.error("Failed to open the extension log stream", error);
        }
        return;
      }

      handleLog = (event: Event) => {
        if (!generationIsCurrent()) return;
        const entry = parseExtensionLogEvent((event as MessageEvent).data);
        if (!entry) return;
        queueSnapshotWrite(current =>
          current
            ? appendExtensionLogEntry(current, entry)
            : { logs: [entry], stream_epoch: entry.stream_epoch }
        );
        store.trigger.streamOpened({ generation });
      };
      handleReset = (event: Event) => {
        if (!generationIsCurrent()) return;
        const snapshot = parseExtensionLogResetEvent((event as MessageEvent).data);
        if (!snapshot) return;
        queueSnapshotWrite(() => snapshot);
        store.trigger.streamOpened({ generation });
      };
      const handleOpen = () => store.trigger.streamOpened({ generation });
      const handleError = () => store.trigger.streamFailed({ generation });
      source.addEventListener(EXTENSION_LOG_EVENT_NAME, handleLog);
      source.addEventListener(EXTENSION_LOG_RESET_EVENT_NAME, handleReset);
      source.onopen = handleOpen;
      source.onerror = handleError;
    };

    void start();
    return () => {
      closed = true;
      if (source) {
        if (handleLog) source.removeEventListener?.(EXTENSION_LOG_EVENT_NAME, handleLog);
        if (handleReset) source.removeEventListener?.(EXTENSION_LOG_RESET_EVENT_NAME, handleReset);
        source.onopen = null;
        source.onerror = null;
        source.close();
      }
      store.trigger.streamSuspended({ generation });
    };
  }, [
    enabled,
    canStream,
    follow,
    instance,
    normalizedName,
    normalizedWorkspaceId,
    queryClient,
    retryToken,
    store,
  ]);

  return {
    entries: history.data?.logs ?? [],
    error: history.error ?? connectionError,
    follow,
    isLoading:
      enabled &&
      normalizedName !== "" &&
      follow &&
      history.data === undefined &&
      history.error === null &&
      status !== "reconnecting",
    refetch: () => {
      if (follow && canStream) store.trigger.retryRequested();
      else void history.refetch();
    },
    setFollow: next => store.trigger.followChanged({ follow: next }),
    status,
  };
}
