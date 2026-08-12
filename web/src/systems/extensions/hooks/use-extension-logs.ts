import { useEffect, useEffectEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { createStreamEventSource } from "@/lib/ticketed-event-source";

import {
  appendExtensionLogEntries,
  buildExtensionLogsStreamUrl,
  extensionLogCursor,
  parseExtensionLogEvent,
} from "../lib/extension-log-stream";
import { extensionLogsOptions } from "../lib/query-options";
import type { ExtensionLogEntry } from "../types";
import { extensionLogsLogic, type ExtensionLogStreamStatus } from "./extension-logs-store";

/** The daemon publishes log records as the named `extension_log` SSE event (never `message`). */
export const EXTENSION_LOG_EVENT_NAME = "extension_log";

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
  entries: readonly ExtensionLogEntry[];
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

/**
 * Initial history comes from the REST read; the follow stream appends on top. Both bind the same
 * `(name, workspace)` instance, and streamed rows are retained across reconnects so an outage never
 * blanks the panel.
 */
export function useExtensionLogs({
  name,
  workspaceId,
  enabled = true,
  eventSourceFactory,
}: UseExtensionLogsOptions): ExtensionLogsModel {
  const instance = `${workspaceId ?? ""}\u0000${name}`;
  const history = useQuery({
    ...extensionLogsOptions(name, { workspaceId }),
    enabled: enabled && name !== "",
  });
  const { store } = useStoreBinding(instance, () => extensionLogsLogic.createStore());
  const streamedEntries = useSelector(store, snapshot => snapshot.context.entries);
  const follow = useSelector(store, snapshot => snapshot.context.follow);
  const streamStatus = useSelector(store, snapshot => snapshot.context.streamStatus);

  const entries = appendExtensionLogEntries(history.data ?? [], streamedEntries);
  const readCursor = useEffectEvent(() => extensionLogCursor(entries));
  const status: ExtensionLogStreamStatus = !follow
    ? "paused"
    : enabled && name !== ""
      ? streamStatus
      : "idle";

  useEffect(() => {
    if (!enabled || name === "" || !follow) {
      return undefined;
    }
    const factory = eventSourceFactory ?? defaultEventSourceFactory;
    if (!eventSourceFactory && typeof EventSource === "undefined") return undefined;
    store.trigger.connecting();
    let source: ExtensionLogEventSource;
    try {
      source = factory(buildExtensionLogsStreamUrl(name, { after: readCursor(), workspaceId }));
    } catch (error) {
      store.trigger.streamFailed();
      console.error("Failed to open the extension log stream", error);
      return undefined;
    }
    const handleLog = (event: Event) => {
      const entry = parseExtensionLogEvent((event as MessageEvent).data);
      if (!entry) return;
      store.trigger.entryReceived({ entry });
    };
    const handleOpen = () => store.trigger.streamOpened();
    const handleError = () => store.trigger.streamFailed();
    source.addEventListener(EXTENSION_LOG_EVENT_NAME, handleLog);
    source.onopen = handleOpen;
    source.onerror = handleError;
    return () => {
      source.removeEventListener?.(EXTENSION_LOG_EVENT_NAME, handleLog);
      source.onopen = null;
      source.onerror = null;
      source.close();
    };
  }, [enabled, eventSourceFactory, follow, instance, name, store, workspaceId]);

  return {
    entries,
    error: history.error ?? null,
    follow,
    isLoading: history.isLoading,
    refetch: () => void history.refetch(),
    setFollow: next => store.trigger.followChanged({ follow: next }),
    status,
  };
}
