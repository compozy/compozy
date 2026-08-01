import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import {
  appendExtensionLogEntries,
  buildExtensionLogsStreamUrl,
  extensionLogCursor,
  parseExtensionLogEvent,
} from "../lib/extension-log-stream";
import { extensionLogsOptions } from "../lib/query-options";
import type { ExtensionLogEntry } from "../types";

/** The daemon publishes log records as the named `extension_log` SSE event (never `message`). */
export const EXTENSION_LOG_EVENT_NAME = "extension_log";

export type ExtensionLogStreamStatus = "idle" | "connecting" | "live" | "reconnecting" | "paused";

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
  return new EventSource(url);
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
  const [streamed, setStreamed] = useState<{ instance: string; entries: ExtensionLogEntry[] }>({
    entries: [],
    instance,
  });
  const [follow, setFollow] = useState(true);
  const [streamStatus, setStreamStatus] = useState<ExtensionLogStreamStatus>("idle");
  const cursorRef = useRef(0);

  const streamedEntries = streamed.instance === instance ? streamed.entries : [];
  const entries = appendExtensionLogEntries(history.data ?? [], streamedEntries);
  const status: ExtensionLogStreamStatus = !follow
    ? "paused"
    : enabled && name !== ""
      ? streamStatus
      : "idle";

  if (streamed.instance !== instance) {
    setStreamed({ entries: [], instance });
  }

  useEffect(() => {
    cursorRef.current = extensionLogCursor(entries);
  }, [entries]);

  useEffect(() => {
    cursorRef.current = 0;
  }, [instance]);

  useEffect(() => {
    if (!enabled || name === "" || !follow) {
      return undefined;
    }
    const factory = eventSourceFactory ?? defaultEventSourceFactory;
    if (!eventSourceFactory && typeof EventSource === "undefined") return undefined;
    setStreamStatus("connecting");
    const source = factory(
      buildExtensionLogsStreamUrl(name, { after: cursorRef.current, workspaceId })
    );
    const handleLog = (event: Event) => {
      const entry = parseExtensionLogEvent((event as MessageEvent).data);
      if (!entry) return;
      setStreamStatus("live");
      setStreamed(current =>
        current.instance === instance
          ? { entries: appendExtensionLogEntries(current.entries, [entry]), instance }
          : { entries: [entry], instance }
      );
    };
    const handleOpen = () => setStreamStatus("live");
    const handleError = () => setStreamStatus("reconnecting");
    source.addEventListener(EXTENSION_LOG_EVENT_NAME, handleLog);
    source.onopen = handleOpen;
    source.onerror = handleError;
    return () => {
      source.removeEventListener?.(EXTENSION_LOG_EVENT_NAME, handleLog);
      source.onopen = null;
      source.onerror = null;
      source.close();
    };
  }, [enabled, eventSourceFactory, follow, instance, name, workspaceId]);

  return {
    entries,
    error: history.error ?? null,
    follow,
    isLoading: history.isLoading,
    refetch: () => void history.refetch(),
    setFollow,
    status,
  };
}
