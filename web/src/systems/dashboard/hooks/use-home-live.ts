import { useEffect, useEffectEvent, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { tasksKeys } from "@/systems/tasks";

import { buildHomeLogsStreamUrl } from "../adapters/overview-api";
import { isTaskLifecycleEvent } from "../lib/activity-classes";
import { homeActivityEventSchema } from "../lib/home-activity-schema";
import { dashboardKeys } from "../lib/query-keys";
import { HOME_ACTIVITY_LIMIT, homeActivityOptions } from "../lib/query-options";
import type { HomeActivityEvent } from "../types";

const PULSE_INVALIDATE_MIN_INTERVAL_MS = 60_000;

interface HomeLiveEventSource {
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

type HomeLiveEventSourceFactory = (url: string) => HomeLiveEventSource;

interface UseHomeLiveOptions {
  workspaceId?: string;
  enabled?: boolean;
  eventSourceFactory?: HomeLiveEventSourceFactory;
  onError?: (error: unknown) => void;
}

function defaultEventSourceFactory(url: string): HomeLiveEventSource {
  return new EventSource(url);
}

type QueryClient = ReturnType<typeof useQueryClient>;

function prependHomeActivityEvent(
  queryClient: QueryClient,
  workspaceId: string,
  event: HomeActivityEvent
) {
  const key = homeActivityOptions({ workspace_id: workspaceId || undefined }).queryKey;
  queryClient.setQueryData<HomeActivityEvent[]>(key, current => {
    if (!current) {
      return current;
    }
    if (current.some(existing => existing.id === event.id)) {
      return current;
    }
    return [event, ...current].slice(0, HOME_ACTIVITY_LIMIT);
  });
}

function invalidateTaskAggregates(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.dashboardRoot() });
  void queryClient.invalidateQueries({ queryKey: tasksKeys.inboxRoot() });
}

/**
 * One logs EventSource keeps the home dashboard live: every event lands in the
 * activity cache; task lifecycle events refresh attention/outcome aggregates
 * immediately; everything else refreshes the overview at most once per minute
 * (the pulse heatmap moves slowly by design).
 */
export function useHomeLive({
  workspaceId = "",
  enabled = true,
  eventSourceFactory: customEventSourceFactory,
  onError,
}: UseHomeLiveOptions = {}) {
  const queryClient = useQueryClient();
  const lastOverviewInvalidateAt = useRef(0);
  const notifyError = useEffectEvent((error: unknown, fallback: string) => {
    if (onError) {
      onError(error);
      return;
    }
    console.error(fallback, error);
  });

  useEffect(() => {
    if (
      !enabled ||
      typeof window === "undefined" ||
      (!customEventSourceFactory && typeof EventSource === "undefined")
    ) {
      return undefined;
    }

    const url = buildHomeLogsStreamUrl(workspaceId);
    const source = (customEventSourceFactory ?? defaultEventSourceFactory)(url);

    source.onmessage = (event: MessageEvent) => {
      if (typeof event.data !== "string") {
        return;
      }
      try {
        const payload = JSON.parse(event.data) as HomeActivityEvent;
        const validation = homeActivityEventSchema.safeParse(payload);
        if (!validation.success) {
          notifyError(validation.error, "Rejected malformed home activity stream payload");
          return;
        }
        prependHomeActivityEvent(queryClient, workspaceId, payload);
        if (isTaskLifecycleEvent(payload)) {
          lastOverviewInvalidateAt.current = Date.now();
          invalidateTaskAggregates(queryClient);
          return;
        }
        const elapsed = Date.now() - lastOverviewInvalidateAt.current;
        if (elapsed >= PULSE_INVALIDATE_MIN_INTERVAL_MS) {
          lastOverviewInvalidateAt.current = Date.now();
          void queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() });
        }
      } catch (error) {
        notifyError(error, "Failed to parse home activity stream payload");
      }
    };
    source.onerror = (event: Event) => {
      notifyError(event, "Home activity stream failed");
    };

    return () => {
      source.onmessage = null;
      source.onerror = null;
      source.close();
    };
  }, [customEventSourceFactory, enabled, queryClient, workspaceId]);
}

export type { HomeLiveEventSource, HomeLiveEventSourceFactory, UseHomeLiveOptions };
