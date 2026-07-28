import { useStore, useSelector } from "@xstate/store-react";
import { useEffect } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import type { WorkspacePayload } from "@/systems/workspace";

import { sessionKeys } from "../lib/query-keys";
import type { SessionCatalogEventPayload } from "../types";
import {
  sessionCatalogStreamsLogic,
  type SessionCatalogStreamStatus,
} from "./session-catalog-streams-store";

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

export type { SessionCatalogStreamStatus } from "./session-catalog-streams-store";

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
  const store = useStore(sessionCatalogStreamsLogic);
  const status = useSelector(store, snapshot => snapshot.context.status);

  useEffect(() => {
    if (!canConnect) {
      store.trigger.connectionDisabled();
      return undefined;
    }

    store.trigger.connectionRequested({
      connect: onStatusChange =>
        openSessionCatalogStream(
          queryClient,
          workspaceKey.split("\u0000"),
          eventSourceFactory ?? defaultEventSourceFactory,
          onStatusChange
        ),
    });
    return () => store.trigger.connectionDisabled();
  }, [canConnect, eventSourceFactory, queryClient, store, workspaceKey]);

  return status;
}
