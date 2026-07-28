import { useEffect } from "react";
import { useQueryClient, type InfiniteData } from "@tanstack/react-query";

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
  return new EventSource(url);
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function buildBridgeHealthStreamUrl(bridgeIds: readonly string[], filters: BridgeListFilter = {}) {
  const params = new URLSearchParams();
  const scope = normalizeOptionalText(filters.scope);
  const workspaceId = normalizeOptionalText(filters.workspace_id);
  const workspace = normalizeOptionalText(filters.workspace);

  params.set("bridge_ids", bridgeIds.join(","));
  if (scope) {
    params.set("scope", scope);
  }
  if (workspaceId) {
    params.set("workspace_id", workspaceId);
  }
  if (workspace) {
    params.set("workspace", workspace);
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

  const cachedRoutes = queryClient.getQueryData<BridgeRoute[] | undefined>(
    bridgeKeys.routes(bridgeID)
  );
  if (cachedRoutes !== undefined && cachedRoutes.length === nextRouteCount) {
    return;
  }

  void queryClient.invalidateQueries({ queryKey: bridgeKeys.routes(bridgeID) });
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
    queryClient.setQueryData<BridgeDetailResponse | undefined>(
      bridgeKeys.detail(bridgeID),
      current =>
        current
          ? {
              ...current,
              health,
            }
          : current
    );
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
  const bridgeIdsKey = [...new Set(bridgeIds)]
    .flatMap(bridgeId => {
      const id = bridgeId.trim();
      return id ? [id] : [];
    })
    .join(",");
  const queryClient = useQueryClient();

  useEffect(() => {
    if (
      !enabled ||
      bridgeIdsKey === "" ||
      typeof window === "undefined" ||
      (!customEventSourceFactory && typeof EventSource === "undefined")
    ) {
      return undefined;
    }

    const bridgeIds = bridgeIdsKey.split(",");
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
        buildBridgeHealthStreamUrl(chunk, { scope, workspace_id: workspaceId, workspace })
      );
      detachSources.push(attachBridgeHealthSource(source, handleSnapshot, handleError));
    }

    return () => {
      for (const detach of detachSources) detach();
    };
  }, [bridgeIdsKey, customEventSourceFactory, enabled, queryClient, scope, workspaceId, workspace]);
}
