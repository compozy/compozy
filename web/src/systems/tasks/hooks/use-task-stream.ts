import { useEffect, useEffectEvent, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { buildTaskStreamUrl } from "../adapters/tasks-api";
import { tasksKeys } from "../lib/query-keys";
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
  return new EventSource(url);
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

type QueryClient = ReturnType<typeof useQueryClient>;

function invalidateLiveTaskStreamQueries(queryClient: QueryClient, taskId: string) {
  void queryClient.invalidateQueries({ queryKey: tasksKeys.detail(taskId) });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.timelineRoot() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.runsRoot() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.runDetails() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.contextBundle() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.agentContext() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.reviewsRoot() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.bridgeNotificationsRoot() });
}

function markTaskCatalogsStale(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: tasksKeys.lists(), refetchType: "none" });
  void queryClient.invalidateQueries({
    queryKey: tasksKeys.dashboardRoot(),
    refetchType: "none",
  });
  void queryClient.invalidateQueries({
    queryKey: tasksKeys.inboxRoot(),
    refetchType: "none",
  });
}

function reconcileActiveTaskCatalogs(queryClient: QueryClient) {
  void queryClient.refetchQueries({ queryKey: tasksKeys.lists(), type: "active" });
  void queryClient.refetchQueries({
    queryKey: tasksKeys.dashboardRoot(),
    type: "active",
  });
  void queryClient.refetchQueries({ queryKey: tasksKeys.inboxRoot(), type: "active" });
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
  const catalogsDirty = useRef(false);
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
    if (!enabled || trimmedId === "") {
      return undefined;
    }
    return () => {
      if (!catalogsDirty.current) {
        return;
      }
      catalogsDirty.current = false;
      reconcileActiveTaskCatalogs(queryClient);
    };
  }, [enabled, queryClient, trimmedId]);

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
        invalidateLiveTaskStreamQueries(queryClient, trimmedId);
        if (!catalogsDirty.current) {
          catalogsDirty.current = true;
          markTaskCatalogsStale(queryClient);
        }
        notifyEvent(payload);
      } catch (error) {
        notifyError(error, "Failed to parse task stream payload");
      }
    };

    const handleError = (event: Event) => {
      notifyError(event, "Task stream failed");
    };

    return attachTaskStreamSource(source, handleMessage, handleError);
  }, [customEventSourceFactory, enabled, filteredAfterSequence, queryClient, trimmedId]);
}

export type { TaskStreamEventSource, TaskStreamEventSourceFactory, UseTaskStreamOptions };
