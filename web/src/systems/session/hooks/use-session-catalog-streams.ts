import { useStore, useSelector } from "@xstate/store-react";
import { useEffect, useRef } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { notificationKeys } from "@/systems/notifications";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";

import { sessionKeys } from "../lib/query-keys";
import { isLiveSessionState } from "../lib/query-options";
import type {
  OperatorNotificationEventPayload,
  SessionAttentionEventPayload,
  SessionCatalogEventPayload,
  SessionPayload,
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
  return createStreamEventSource(url, { transport: "websocket", resumeWithLastEventId: true });
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

// One wake window per stream, independent of the number of lifecycle/attention frames.
// An in-flight read is allowed to finish; cancelling it on every frame amplified
// one background transition into repeated full-catalog walks.
function createCatalogReconciler(queryClient: QueryClient) {
  const workspaces = new Set<string>();
  const pending = new Set<string>();
  let lastRead = -Infinity;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let focused = true;
  const visible = () =>
    focused && (typeof document === "undefined" || document.visibilityState !== "hidden");
  const flush = () => {
    timer = undefined;
    if ((workspaces.size === 0 && pending.size === 0) || !visible()) return;
    lastRead = Date.now();
    const hashes = new Set(pending);
    pending.clear();
    const prefixes = [
      ...Array.from(workspaces, workspace => sessionKeys.workspaceLists(workspace)),
      ...(workspaces.size > 0
        ? [sessionKeys.attentionSummary(), notificationKeys.attentionRoot()]
        : []),
    ];
    workspaces.clear();
    for (const queryKey of prefixes) {
      for (const query of queryClient.getQueryCache().findAll({ queryKey })) {
        hashes.delete(query.queryHash);
      }
    }
    // A retry retains only blocked identities. Healthy siblings require a new wake.
    const predicate = (query: {
      queryHash: string;
      state: { status: string; errorUpdatedAt: number; fetchStatus: string };
      getObserversCount: () => number;
    }) => {
      const blocked =
        query.getObserversCount() > 0 &&
        (query.state.fetchStatus === "fetching" ||
          (query.state.status === "error" && Date.now() - query.state.errorUpdatedAt < 30_000));
      if (blocked) pending.add(query.queryHash);
      return !blocked;
    };
    for (const queryKey of prefixes) {
      void queryClient.invalidateQueries({ queryKey, predicate }, { cancelRefetch: false });
    }
    if (hashes.size > 0) {
      void queryClient.invalidateQueries(
        { predicate: query => hashes.has(query.queryHash) && predicate(query) },
        { cancelRefetch: false }
      );
    }
    schedule();
  };
  const schedule = () => {
    if (timer !== undefined || !visible() || (workspaces.size === 0 && pending.size === 0)) return;
    const delay = Math.max(0, 5_000 - (Date.now() - lastRead));
    if (delay === 0) flush();
    else timer = setTimeout(flush, delay);
  };
  const onVisibility = () => schedule();
  const onFocus = () => {
    focused = true;
    schedule();
  };
  const onBlur = () => {
    focused = false;
  };
  if (typeof window !== "undefined") {
    window.addEventListener("focus", onFocus);
    window.addEventListener("blur", onBlur);
  }
  if (typeof document !== "undefined") document.addEventListener("visibilitychange", onVisibility);
  return {
    wake(workspace?: string) {
      workspaces.add("");
      if (workspace !== undefined) workspaces.add(workspace);
      schedule();
    },
    close() {
      if (typeof window !== "undefined") {
        window.removeEventListener("focus", onFocus);
        window.removeEventListener("blur", onBlur);
      }
      if (timer !== undefined) clearTimeout(timer);
      if (typeof document !== "undefined")
        document.removeEventListener("visibilitychange", onVisibility);
    },
  };
}

/**
 * Re-reads the session detail, then wakes a stopped session's mounted transcript.
 *
 * A catalog upsert is the only signal for a transcript entry appended after the
 * session stopped (the post-stop discard marker): the transcript stream has already
 * ended on the terminal frame and a stopped session never polls. The decision is
 * taken from the authoritative detail read, never from the cached state — the wake
 * can arrive while the cache still says active/stopping although the daemon has
 * already stopped — so the transcript is re-read only after the detail invalidation
 * settles as non-live. Live sessions keep the live tail as the transcript's sole
 * owner, and an unknown state is left to the live tail's own detail read.
 */
async function refreshSessionDetail(
  queryClient: QueryClient,
  payload: SessionCatalogEventPayload
): Promise<void> {
  const detailKey = sessionKeys.detail(payload.workspace_id, payload.session_id);
  await queryClient.invalidateQueries({ queryKey: detailKey, exact: true });
  if (payload.kind !== "upserted") return;
  const state = queryClient.getQueryData<SessionPayload>(detailKey)?.state;
  if (state === undefined || isLiveSessionState(state)) return;
  await queryClient.invalidateQueries({
    queryKey: sessionKeys.transcript(payload.workspace_id, payload.session_id),
    exact: true,
  });
}

function openSessionCatalogStream(
  queryClient: QueryClient,
  eventSourceFactory: SessionCatalogEventSourceFactory,
  onStatusChange: (status: Exclude<SessionCatalogStreamStatus, "disabled">) => void,
  handlers: SessionCatalogStreamHandlers,
  url: string
): () => void {
  const reconciler = createCatalogReconciler(queryClient);
  const reconcileWorkspaces: EventListener = () => {
    onStatusChange("live");
    reconciler.wake();
  };
  const handleStreamError: EventListener = () => onStatusChange("stale");
  const handleCatalogChange: EventListener = event => {
    const payload = parseSessionCatalogEvent(event);
    if (!payload) return;
    reconciler.wake(payload.workspace_id);
    void refreshSessionDetail(queryClient, payload);
    // Same session, whichever lens is holding it open.
    void queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(payload.session_id) });
    // Every subagent transition upserts its parent: a mounted roster or chip
    // preview re-reads the list route (`_spec.md` §Catalog liveness).
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.subagents(payload.workspace_id, payload.session_id),
    });
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
    reconciler.wake(payload.workspace_id);
    void queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(payload.session_id) });
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
    reconciler.close();
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
    (eventSourceFactory !== undefined || typeof WebSocket !== "undefined");
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
