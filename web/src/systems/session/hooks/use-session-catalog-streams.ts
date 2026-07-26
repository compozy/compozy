import { useEffect, useState, useSyncExternalStore } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import type { WorkspacePayload } from "@/systems/workspace";

import { sessionKeys } from "../lib/query-keys";
import type { SessionCatalogEventPayload } from "../types";

const SESSION_CATALOG_CHANGED_EVENT = "session_catalog_changed";

export interface SessionCatalogEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
}

export type SessionCatalogEventSourceFactory = (url: string) => SessionCatalogEventSource;

interface UseSessionCatalogStreamsOptions {
  enabled?: boolean;
  eventSourceFactory?: SessionCatalogEventSourceFactory;
}

export type SessionCatalogStreamStatus = "disabled" | "connecting" | "live" | "stale";

interface SessionCatalogStatusStore {
  getSnapshot: () => SessionCatalogStreamStatus;
  subscribe: (listener: () => void) => () => void;
  set: (status: SessionCatalogStreamStatus) => void;
}

function createSessionCatalogStatusStore(
  initialStatus: SessionCatalogStreamStatus
): SessionCatalogStatusStore {
  let status = initialStatus;
  const listeners = new Set<() => void>();
  return {
    getSnapshot: () => status,
    subscribe: listener => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    set: nextStatus => {
      if (nextStatus === status) return;
      status = nextStatus;
      for (const listener of listeners) listener();
    },
  };
}

export function sessionCatalogStreamURL(): string {
  return "/api/sessions/catalog-stream";
}

function defaultEventSourceFactory(url: string): SessionCatalogEventSource {
  return new EventSource(url);
}

function parseSessionCatalogEvent(event: Event): SessionCatalogEventPayload | undefined {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") return undefined;
  try {
    const payload = JSON.parse(event.data) as Partial<SessionCatalogEventPayload>;
    if (
      (payload.kind !== "upserted" && payload.kind !== "deleted") ||
      typeof payload.workspace_id !== "string" ||
      typeof payload.session_id !== "string" ||
      payload.workspace_id.trim() === "" ||
      payload.session_id.trim() === ""
    ) {
      return undefined;
    }
    return payload as SessionCatalogEventPayload;
  } catch {
    return undefined;
  }
}

function openSessionCatalogStream(
  queryClient: QueryClient,
  workspaceIds: readonly string[],
  eventSourceFactory: SessionCatalogEventSourceFactory,
  onStatusChange: (status: Exclude<SessionCatalogStreamStatus, "disabled">) => void
): () => void {
  const authorizedWorkspaceIds = new Set(workspaceIds);
  const reconcileWorkspaces: EventListener = () => {
    onStatusChange("live");
    for (const workspaceId of authorizedWorkspaceIds) {
      void queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) });
    }
  };
  const handleStreamError: EventListener = () => onStatusChange("stale");
  const handleCatalogChange: EventListener = event => {
    const payload = parseSessionCatalogEvent(event);
    if (!payload || !authorizedWorkspaceIds.has(payload.workspace_id)) return;
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.workspaceLists(payload.workspace_id),
    });
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.detail(payload.workspace_id, payload.session_id),
      exact: true,
    });
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.byId(payload.session_id),
      exact: true,
    });
  };
  const source = eventSourceFactory(sessionCatalogStreamURL());
  try {
    source.addEventListener("open", reconcileWorkspaces);
    source.addEventListener("error", handleStreamError);
    source.addEventListener(SESSION_CATALOG_CHANGED_EVENT, handleCatalogChange);
  } catch (error) {
    source.removeEventListener("open", reconcileWorkspaces);
    source.removeEventListener("error", handleStreamError);
    source.removeEventListener(SESSION_CATALOG_CHANGED_EVENT, handleCatalogChange);
    source.close();
    throw error;
  }

  return () => {
    source.removeEventListener("open", reconcileWorkspaces);
    source.removeEventListener("error", handleStreamError);
    source.removeEventListener(SESSION_CATALOG_CHANGED_EVENT, handleCatalogChange);
    source.close();
  };
}

export function useSessionCatalogStreams(
  workspaces: readonly WorkspacePayload[],
  { enabled = true, eventSourceFactory }: UseSessionCatalogStreamsOptions = {}
) {
  const queryClient = useQueryClient();
  const workspaceIds = [
    ...new Set(
      workspaces.flatMap(workspace => {
        const workspaceId = workspace.id.trim();
        return workspaceId === "" ? [] : [workspaceId];
      })
    ),
  ];
  const workspaceKey = workspaceIds.join("\u0000");
  const canConnect =
    enabled &&
    workspaceKey !== "" &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");
  const [statusStore] = useState(() =>
    createSessionCatalogStatusStore(canConnect ? "connecting" : "disabled")
  );
  const status = useSyncExternalStore(
    statusStore.subscribe,
    statusStore.getSnapshot,
    statusStore.getSnapshot
  );

  useEffect(() => {
    if (!canConnect) {
      statusStore.set("disabled");
      return undefined;
    }

    statusStore.set("connecting");
    try {
      const close = openSessionCatalogStream(
        queryClient,
        workspaceKey.split("\u0000"),
        eventSourceFactory ?? defaultEventSourceFactory,
        statusStore.set
      );
      return close;
    } catch {
      statusStore.set("stale");
      return undefined;
    }
  }, [canConnect, eventSourceFactory, queryClient, statusStore, workspaceKey]);

  return status;
}
