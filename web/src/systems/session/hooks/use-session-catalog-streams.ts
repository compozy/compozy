import { useStore, useSelector } from "@xstate/store-react";
import { useEffect, useRef } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";

import { sessionKeys } from "../lib/query-keys";
import type {
  OperatorNotificationEventPayload,
  SessionAttentionEventPayload,
  SessionCatalogEventPayload,
} from "../types";
import {
  sessionCatalogStreamsLogic,
  type SessionCatalogStreamStatus,
} from "./session-catalog-streams-store";

const SESSION_CATALOG_CHANGED_EVENT = "session_catalog_changed";
/** Transition edges — ephemeral delivery signals, never a state patch. */
const SESSION_ATTENTION_CHANGED_EVENT = "session_attention_changed";
/** Sanitized agent-sent notifications riding the same stream. */
const OPERATOR_NOTIFICATION_EVENT = "operator_notification";

export interface SessionCatalogEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
}

export type SessionCatalogEventSourceFactory = (url: string) => SessionCatalogEventSource;

interface UseSessionCatalogStreamsOptions extends SessionCatalogStreamHandlers {
  enabled?: boolean;
  eventSourceFactory?: SessionCatalogEventSourceFactory;
}

export type { SessionCatalogStreamStatus } from "./session-catalog-streams-store";

export function sessionCatalogStreamURL(scope: ProfileScopeParams): string {
  const params = new URLSearchParams({ all_workspaces: "true" });
  if ("all_profiles" in scope) params.set("all_profiles", "true");
  else params.set("profile", scope.profile);
  return `/api/sessions/catalog-stream?${params.toString()}`;
}

function defaultEventSourceFactory(url: string): SessionCatalogEventSource {
  return createStreamEventSource(url);
}

function parseSessionCatalogEvent(event: Event): SessionCatalogEventPayload | undefined {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") return undefined;
  try {
    const payload = JSON.parse(event.data) as Partial<SessionCatalogEventPayload>;
    if (
      (payload.kind !== "upserted" && payload.kind !== "deleted") ||
      typeof payload.workspace_id !== "string" ||
      typeof payload.session_id !== "string" ||
      payload.session_id.trim() === ""
    ) {
      return undefined;
    }
    return payload as SessionCatalogEventPayload;
  } catch {
    return undefined;
  }
}

function parseNamedEvent<T extends object>(
  event: Event,
  required: readonly (keyof T & string)[]
): T | undefined {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") return undefined;
  try {
    const payload = JSON.parse(event.data) as Record<string, unknown>;
    for (const key of required) {
      const value = payload[key];
      if (typeof value !== "string" || value.trim() === "") return undefined;
    }
    return payload as T;
  } catch {
    return undefined;
  }
}

export interface SessionCatalogStreamHandlers {
  onAttentionEdge?: (edge: SessionAttentionEventPayload) => void;
  onOperatorNotification?: (notification: OperatorNotificationEventPayload) => void;
}

function invalidateGlobalSessionViews(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists("") });
  void queryClient.invalidateQueries({ queryKey: sessionKeys.attentionSummary() });
}

function openSessionCatalogStream(
  queryClient: QueryClient,
  eventSourceFactory: SessionCatalogEventSourceFactory,
  onStatusChange: (status: Exclude<SessionCatalogStreamStatus, "disabled">) => void,
  handlers: SessionCatalogStreamHandlers,
  url: string
): () => void {
  const reconcileWorkspaces: EventListener = () => {
    onStatusChange("live");
    invalidateGlobalSessionViews(queryClient);
  };
  const handleStreamError: EventListener = () => onStatusChange("stale");
  const handleCatalogChange: EventListener = event => {
    const payload = parseSessionCatalogEvent(event);
    if (!payload) return;
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.workspaceLists(payload.workspace_id),
    });
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.detail(payload.workspace_id, payload.session_id),
      exact: true,
    });
    // Same session, whichever lens is holding it open.
    void queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(payload.session_id) });
    invalidateGlobalSessionViews(queryClient);
  };
  const handleAttentionEdge: EventListener = event => {
    const payload = parseNamedEvent<SessionAttentionEventPayload>(event, [
      "session_id",
      "workspace_id",
      "from",
      "to",
      "class",
      "at",
    ]);
    if (!payload) return;
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.workspaceLists(payload.workspace_id),
    });
    invalidateGlobalSessionViews(queryClient);
    handlers.onAttentionEdge?.(payload);
  };
  const handleOperatorNotification: EventListener = event => {
    const payload = parseNamedEvent<OperatorNotificationEventPayload>(event, [
      "notification_id",
      "session_id",
      "workspace_id",
      "title",
      "at",
    ]);
    if (!payload) return;
    handlers.onOperatorNotification?.(payload);
  };
  const listeners: Array<[string, EventListener]> = [
    ["open", reconcileWorkspaces],
    ["error", handleStreamError],
    [SESSION_CATALOG_CHANGED_EVENT, handleCatalogChange],
    [SESSION_ATTENTION_CHANGED_EVENT, handleAttentionEdge],
    [OPERATOR_NOTIFICATION_EVENT, handleOperatorNotification],
  ];
  const source = eventSourceFactory(url);
  const detach = () => {
    for (const [type, listener] of listeners) source.removeEventListener(type, listener);
  };
  try {
    for (const [type, listener] of listeners) source.addEventListener(type, listener);
  } catch (error) {
    detach();
    source.close();
    throw error;
  }

  return () => {
    detach();
    source.close();
  };
}

export function useSessionCatalogStreams({
  enabled = true,
  eventSourceFactory,
  onAttentionEdge,
  onOperatorNotification,
}: UseSessionCatalogStreamsOptions = {}) {
  const queryClient = useQueryClient();
  // Handlers ride a ref so a re-rendering consumer never reopens the stream.
  // Committed in an effect, not during render: a discarded render must not
  // point the live stream at a handler that never reached the screen.
  const handlersRef = useRef<SessionCatalogStreamHandlers>({});
  useEffect(() => {
    handlersRef.current = { onAttentionEdge, onOperatorNotification };
  });
  const canConnect =
    enabled &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");
  const store = useStore(sessionCatalogStreamsLogic);
  const status = useSelector(store, snapshot => snapshot.context.status);
  // The URL carries the profile scope, so it doubles as the reconnect identity:
  // a switch changes the string, which bumps the store's generation, fences every
  // frame still in flight from the old socket, and closes it before the new one
  // opens — leaving a profile's scope stops its stream writes (US-010.EC-2).
  const streamUrl = sessionCatalogStreamURL(useProfileReadScope().params);

  useEffect(() => {
    if (!canConnect) {
      store.trigger.connectionDisabled();
      return undefined;
    }

    store.trigger.connectionRequested({
      connect: onStatusChange => {
        return openSessionCatalogStream(
          queryClient,
          eventSourceFactory ?? defaultEventSourceFactory,
          onStatusChange,
          {
            onAttentionEdge: edge => handlersRef.current.onAttentionEdge?.(edge),
            onOperatorNotification: notification =>
              handlersRef.current.onOperatorNotification?.(notification),
          },
          streamUrl
        );
      },
    });
    return () => store.trigger.connectionDisabled();
  }, [canConnect, eventSourceFactory, queryClient, store, streamUrl]);

  return status;
}
