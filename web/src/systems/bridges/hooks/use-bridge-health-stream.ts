import { useEffect } from "react";
import { useQueryClient, type InfiniteData } from "@tanstack/react-query";

import { createStreamEventSource } from "@/lib/ticketed-event-source";
import { useProfileReadScope } from "@/systems/profiles";

import { bridgeKeys } from "../lib/query-keys";
import type {
  BridgeDetailResponse,
  BridgeListFilter,
  BridgeRoute,
  BridgesListResponse,
  BridgeHealthStreamSnapshot,
} from "../types";

interface BridgeHealthEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
  onerror: ((event: Event) => void) | null;
  removeEventListener?: (type: string, listener: EventListenerOrEventListenerObject) => void;
}

interface UseBridgeHealthStreamOptions {
  bridgeIds?: readonly string[];
  enabled?: boolean;
  eventSourceFactory?: (url: string) => BridgeHealthEventSource;
  filters?: BridgeListFilter;
}

const BRIDGE_HEALTH_STREAM_URL = "/api/bridges/health/stream";
const MAX_BRIDGE_IDS_PER_STREAM = 200;

function defaultEventSourceFactory(url: string): BridgeHealthEventSource {
  return createStreamEventSource(url);
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function optionalOpaqueIdentity(value?: string | null): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

function buildBridgeHealthStreamUrl(bridgeIds: readonly string[], filters: BridgeListFilter = {}) {
  const params = new URLSearchParams();
  const scope = normalizeOptionalText(filters.scope);
  const workspaceId = optionalOpaqueIdentity(filters.workspace_id);
  const workspace = normalizeOptionalText(filters.workspace);

  for (const bridgeId of bridgeIds) {
    params.append("bridge_ids", bridgeId);
  }
  if (scope) {
    params.set("scope", scope);
  }
  if (workspaceId) {
    params.set("workspace_id", workspaceId);
  }
  if (workspace) {
    params.set("workspace", workspace);
  }
  if (filters.profile) {
    params.set("profile", filters.profile);
  }
  if (filters.all_profiles === true) {
    params.set("all_profiles", "true");
  }

  const query = params.toString();
  return query ? `${BRIDGE_HEALTH_STREAM_URL}?${query}` : BRIDGE_HEALTH_STREAM_URL;
}

function invalidateBridgeRoutesWhenCountChanges(
  queryClient: ReturnType<typeof useQueryClient>,
  bridgeID: string,
  nextRouteCount: number | undefined
) {
  if (nextRouteCount === undefined) {
    return;
  }

  // Route count is the same fact under every lens, so agreement has to hold for
  // all of them before the reread can be skipped.
  const cached = queryClient.getQueriesData<BridgeRoute[] | undefined>({
    queryKey: bridgeKeys.routesFor(bridgeID),
  });
  const settled =
    cached.length > 0 &&
    cached.every(([, routes]) => routes !== undefined && routes.length === nextRouteCount);
  if (settled) {
    return;
  }

  void queryClient.invalidateQueries({ queryKey: bridgeKeys.routesFor(bridgeID) });
}

function mergeBridgeHealthSnapshotPage(
  current: BridgesListResponse,
  snapshot: BridgeHealthStreamSnapshot
): BridgesListResponse {
  const visibleBridgeIds = new Set(current.bridges.map(bridge => bridge.id));
  const incomingHealth = Object.fromEntries(
    Object.entries(snapshot.bridge_health).filter(([bridgeID]) => visibleBridgeIds.has(bridgeID))
  );
  return {
    ...current,
    bridge_health: { ...current.bridge_health, ...incomingHealth },
  };
}

function mergeBridgeHealthSnapshot(
  current: InfiniteData<BridgesListResponse, unknown> | undefined,
  snapshot: BridgeHealthStreamSnapshot
): InfiniteData<BridgesListResponse, unknown> | undefined {
  if (!current) return current;
  return {
    ...current,
    pages: current.pages.map(page => mergeBridgeHealthSnapshotPage(page, snapshot)),
  };
}

export function applyBridgeHealthSnapshot(
  queryClient: ReturnType<typeof useQueryClient>,
  snapshot: BridgeHealthStreamSnapshot
) {
  for (const [queryKey] of queryClient.getQueriesData<
    InfiniteData<BridgesListResponse, unknown> | undefined
  >({
    queryKey: bridgeKeys.lists(),
  })) {
    queryClient.setQueryData<InfiniteData<BridgesListResponse, unknown> | undefined>(
      queryKey,
      current => mergeBridgeHealthSnapshot(current, snapshot)
    );
  }

  for (const [bridgeID, health] of Object.entries(snapshot.bridge_health)) {
    // Health is lens-independent, so every lens that already holds this bridge
    // gets the same update rather than one going stale behind the other.
    for (const [queryKey] of queryClient.getQueriesData<BridgeDetailResponse | undefined>({
      queryKey: bridgeKeys.detailFor(bridgeID),
    })) {
      queryClient.setQueryData<BridgeDetailResponse | undefined>(queryKey, current =>
        current ? { ...current, health } : current
      );
    }
    invalidateBridgeRoutesWhenCountChanges(queryClient, bridgeID, health.route_count);
  }
}

function attachBridgeHealthSource(
  source: BridgeHealthEventSource,
  handleSnapshot: (event: Event) => void,
  handleError: (event: Event) => void
): () => void {
  source.addEventListener("snapshot", handleSnapshot);
  source.onerror = handleError;
  return () => {
    source.removeEventListener?.("snapshot", handleSnapshot);
    source.onerror = null;
    source.close();
  };
}

export function useBridgeHealthStream({
  bridgeIds = [],
  enabled = true,
  eventSourceFactory: customEventSourceFactory,
  filters: { scope, workspace_id: workspaceId, workspace } = {},
}: UseBridgeHealthStreamOptions = {}) {
  const { params: profileScope } = useProfileReadScope();
  const profile = "profile" in profileScope ? profileScope.profile : undefined;
  const allProfiles = "all_profiles" in profileScope ? profileScope.all_profiles : undefined;
  const bridgeIdsKey = JSON.stringify([...new Set(bridgeIds)].filter(bridgeId => bridgeId !== ""));
  const queryClient = useQueryClient();

  useEffect(() => {
    if (
      !enabled ||
      bridgeIdsKey === "[]" ||
      typeof window === "undefined" ||
      (!customEventSourceFactory && typeof EventSource === "undefined")
    ) {
      return undefined;
    }

    const bridgeIds = JSON.parse(bridgeIdsKey) as string[];
    const detachSources: Array<() => void> = [];
    const handleSnapshot = (event: Event) => {
      if (!(event instanceof MessageEvent) || typeof event.data !== "string") {
        return;
      }

      try {
        const snapshot = JSON.parse(event.data) as BridgeHealthStreamSnapshot;
        applyBridgeHealthSnapshot(queryClient, snapshot);
      } catch (error) {
        console.error("Failed to parse bridge health snapshot", error);
      }
    };

    const handleError = (event: Event) => {
      console.error("Bridge health stream failed", event);
    };

    for (let index = 0; index < bridgeIds.length; index += MAX_BRIDGE_IDS_PER_STREAM) {
      const chunk = bridgeIds.slice(index, index + MAX_BRIDGE_IDS_PER_STREAM);
      const source = (customEventSourceFactory ?? defaultEventSourceFactory)(
        buildBridgeHealthStreamUrl(chunk, {
          scope,
          workspace_id: workspaceId,
          workspace,
          profile,
          all_profiles: allProfiles,
        })
      );
      detachSources.push(attachBridgeHealthSource(source, handleSnapshot, handleError));
    }

    return () => {
      for (const detach of detachSources) detach();
    };
  }, [
    bridgeIdsKey,
    customEventSourceFactory,
    enabled,
    allProfiles,
    queryClient,
    profile,
    scope,
    workspaceId,
    workspace,
  ]);
}
