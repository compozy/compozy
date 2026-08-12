import { useEffect, useEffectEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { createStreamEventSource } from "@/lib/ticketed-event-source";
import { useStoreBinding } from "@/hooks/use-store-binding";

import { buildTaskStreamUrl } from "../adapters/tasks-api";
import { createTaskLiveRefreshStore } from "../lib/task-live-refresh-coordinator";
import type { TaskStreamFilter, TaskStreamPayload } from "../types";

interface TaskStreamEventSource {
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

type TaskStreamEventSourceFactory = (url: string) => TaskStreamEventSource;

interface UseTaskStreamOptions {
  enabled?: boolean;
  afterSequence?: number;
  filters?: TaskStreamFilter;
  eventSourceFactory?: TaskStreamEventSourceFactory;
  onEvent?: (payload: TaskStreamPayload) => void;
  onError?: (error: unknown) => void;
}

function defaultEventSourceFactory(url: string): TaskStreamEventSource {
  return createStreamEventSource(url);
}

function attachTaskStreamSource(
  source: TaskStreamEventSource,
  handleMessage: (event: MessageEvent) => void,
  handleError: (event: Event) => void
): () => void {
  source.onmessage = handleMessage;
  source.onerror = handleError;
  return () => {
    source.onmessage = null;
    source.onerror = null;
    source.close();
  };
}

export function useTaskStream(
  taskId: string,
  {
    enabled = true,
    afterSequence: fallbackAfterSequence,
    filters: { after_sequence: filteredAfterSequence = fallbackAfterSequence } = {},
    eventSourceFactory: customEventSourceFactory,
    onEvent,
    onError,
  }: UseTaskStreamOptions = {}
) {
  const queryClient = useQueryClient();
  const trimmedId = taskId.trim();
  const refreshBindingKey = { enabled, queryClient, taskId: trimmedId };
  const { store: refreshStore } = useStoreBinding(
    refreshBindingKey,
    () => createTaskLiveRefreshStore(queryClient, trimmedId),
    () => createTaskLiveRefreshStore(queryClient, trimmedId),
    (current, next) =>
      current.key.enabled !== next.enabled ||
      current.key.queryClient !== next.queryClient ||
      current.key.taskId !== next.taskId
  );
  const notifyEvent = useEffectEvent((payload: TaskStreamPayload) => {
    onEvent?.(payload);
  });
  const notifyError = useEffectEvent((error: unknown, fallback: string) => {
    if (onError) {
      onError(error);
      return;
    }
    console.error(fallback, error);
  });

  useEffect(() => {
    return () => refreshStore.trigger.disposed();
  }, [refreshStore]);

  useEffect(() => {
    if (
      !enabled ||
      trimmedId === "" ||
      typeof window === "undefined" ||
      (!customEventSourceFactory && typeof EventSource === "undefined")
    ) {
      return undefined;
    }

    const url = buildTaskStreamUrl(trimmedId, { after_sequence: filteredAfterSequence });
    const source = (customEventSourceFactory ?? defaultEventSourceFactory)(url);

    const handleMessage = (event: MessageEvent) => {
      if (typeof event.data !== "string") {
        return;
      }
      try {
        const payload = JSON.parse(event.data) as TaskStreamPayload;
        refreshStore.trigger.refreshRequested();
        notifyEvent(payload);
      } catch (error) {
        notifyError(error, "Failed to parse task stream payload");
      }
    };

    const handleError = (event: Event) => {
      notifyError(event, "Task stream failed");
    };

    return attachTaskStreamSource(source, handleMessage, handleError);
  }, [customEventSourceFactory, enabled, filteredAfterSequence, refreshStore, trimmedId]);
}

export type { TaskStreamEventSource, TaskStreamEventSourceFactory, UseTaskStreamOptions };
