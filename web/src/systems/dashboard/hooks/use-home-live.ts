import { useEffect, useEffectEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useStore } from "@xstate/store-react";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { buildHomeLogsStreamUrl } from "../adapters/overview-api";
import { isTaskLifecycleEvent } from "../lib/activity-classes";
import { homeActivityEventSchema } from "../lib/home-activity-schema";
import { dashboardKeys } from "../lib/query-keys";
import { HOME_ACTIVITY_LIMIT, homeActivityOptions } from "../lib/query-options";
import type { HomeActivityEvent, HomeActivityFilter } from "../types";
import { homeLiveRefreshLogic } from "./home-live-refresh-store";
import { PROFILE_AGGREGATE, type ProfileScopeParams } from "@/systems/profiles";
import { tasksKeys } from "@/systems/tasks";

const PULSE_INVALIDATE_MIN_INTERVAL_MS = 60_000;

interface HomeLiveEventSource {
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

type HomeLiveEventSourceFactory = (url: string) => HomeLiveEventSource;

interface UseHomeLiveOptions {
  workspaceId?: string;
  /**
   * The lens the home surface is reading. Required rather than defaulted: a
   * stream that guesses its scope would quietly feed one profile's events into
   * another's list.
   */
  scope: ProfileScopeParams;
  enabled?: boolean;
  eventSourceFactory?: HomeLiveEventSourceFactory;
  onError?: (error: unknown) => void;
}

function lensKeyOf(scope: ProfileScopeParams): string {
  return "all_profiles" in scope ? PROFILE_AGGREGATE : scope.profile;
}

/** The exact filter the activity query uses, so the stream writes into the
 * entry the list is reading rather than a sibling of it. */
function activityFilterFor(workspaceId: string, lensKey: string): HomeActivityFilter {
  return {
    workspace_id: workspaceId || undefined,
    ...(lensKey === PROFILE_AGGREGATE ? { all_profiles: true } : { profile: lensKey }),
  };
}

function defaultEventSourceFactory(url: string): HomeLiveEventSource {
  return createStreamEventSource(url);
}

type QueryClient = ReturnType<typeof useQueryClient>;

function prependHomeActivityEvent(
  queryClient: QueryClient,
  filters: HomeActivityFilter,
  event: HomeActivityEvent
) {
  const key = homeActivityOptions(filters).queryKey;
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

function invalidateTaskAggregates(queryClient: QueryClient): Promise<unknown[]> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.dashboardRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.inboxRoot() }),
  ]);
}

/**
 * One logs EventSource keeps the home dashboard live: every event lands in the
 * activity cache; task lifecycle events refresh attention/outcome aggregates
 * immediately; everything else refreshes the overview at most once per minute
 * (the pulse heatmap moves slowly by design).
 */
export function useHomeLive({
  workspaceId = "",
  scope,
  enabled = true,
  eventSourceFactory: customEventSourceFactory,
  onError,
}: UseHomeLiveOptions) {
  const queryClient = useQueryClient();
  const lensKey = lensKeyOf(scope);
  const url = buildHomeLogsStreamUrl(workspaceId, scope);
  const refreshStore = useStore(homeLiveRefreshLogic);
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

    const source = (customEventSourceFactory ?? defaultEventSourceFactory)(url);
    const activityFilter = activityFilterFor(workspaceId, lensKey);
    // The url identifies workspace and lens together, so a frame still in flight
    // from the source a switch just replaced is fenced out by its own scope.
    refreshStore.trigger.scopeActivated({ scope: url });

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
        prependHomeActivityEvent(queryClient, activityFilter, payload);
        refreshStore.trigger.activityReceived({
          at: Date.now(),
          invalidateOverview: () =>
            queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() }),
          invalidateTaskAggregates: () => invalidateTaskAggregates(queryClient),
          lifecycle: isTaskLifecycleEvent(payload),
          minimumIntervalMs: PULSE_INVALIDATE_MIN_INTERVAL_MS,
          scope: url,
        });
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
    // `url` is the stream's identity: changing lens or workspace rebuilds it,
    // which tears the previous source down before the next one opens.
  }, [customEventSourceFactory, enabled, lensKey, queryClient, refreshStore, url, workspaceId]);
}

export type { HomeLiveEventSource, HomeLiveEventSourceFactory, UseHomeLiveOptions };
