import { useStore, useSelector } from "@xstate/store-react";
import { useEffect, useRef } from "react";
import { useQueryClient, type QueryClient, type QueryKey } from "@tanstack/react-query";

import { notificationKeys } from "@/systems/notifications";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";

import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMonitorsQueries } from "../lib/session-query-invalidation";
import { createSessionReadReconciler } from "./session-catalog-reconciliation";
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

// Covers multiple transport backoff attempts without introducing another read.
const SESSION_CATALOG_DISCONNECT_GRACE_MS = 10_000;

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
    // A folder or manual-order change names its workspace but no session.
    if (payload.kind === "layout") {
      return typeof payload.workspace_id === "string" && payload.workspace_id.trim() !== ""
        ? (payload as SessionCatalogEventPayload)
        : undefined;
    }
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

/** Trailing window for a parent's cold subagent list re-read. */
export const SUBAGENT_LIST_WAKE_MS = 1_000;

/**
 * Every subagent transition upserts its parent (`_spec.md` §Catalog liveness).
 * A burst of upserts for one parent collapses into one trailing re-read of its
 * cold list (chip preview or an inspector without a live roster); a parent
 * whose session stream is open reads its roster instead, so this stays idle.
 */
function createSubagentListReconciler(queryClient: QueryClient) {
  const timers = new Map<string, ReturnType<typeof setTimeout>>();
  return {
    wake(workspaceId: string, sessionId: string) {
      const queryKey = sessionKeys.subagents(workspaceId, sessionId);
      const id = JSON.stringify(queryKey);
      if (timers.has(id)) return;
      timers.set(
        id,
        setTimeout(() => {
          timers.delete(id);
          void queryClient.invalidateQueries({ queryKey }, { cancelRefetch: false });
        }, SUBAGENT_LIST_WAKE_MS)
      );
    },
    close() {
      for (const timer of timers.values()) clearTimeout(timer);
      timers.clear();
    },
  };
}

function isSessionMonitorsQuery(query: { queryKey: QueryKey }): boolean {
  const key = query.queryKey;
  return key[0] === sessionKeys.all[0] && key.at(-1) === "monitors" && key.includes("detail");
}

function openSessionCatalogStream(
  queryClient: QueryClient,
  eventSourceFactory: SessionCatalogEventSourceFactory,
  onStatusChange: (status: Exclude<SessionCatalogStreamStatus, "disabled">) => void,
  handlers: SessionCatalogStreamHandlers,
  url: string
): () => void {
  const reconciler = createCatalogReconciler(queryClient);
  const subagentLists = createSubagentListReconciler(queryClient);
  const sessionReads = createSessionReadReconciler(queryClient);
  let staleTimer: ReturnType<typeof setTimeout> | undefined;
  const clearStaleTimer = () => {
    if (staleTimer !== undefined) clearTimeout(staleTimer);
    staleTimer = undefined;
  };
  const markLive = () => {
    clearStaleTimer();
    onStatusChange("live");
  };
  const handleStreamError = () => {
    if (staleTimer !== undefined) return;
    staleTimer = setTimeout(() => {
      staleTimer = undefined;
      onStatusChange("stale");
    }, SESSION_CATALOG_DISCONNECT_GRACE_MS);
  };
  const reconcileWorkspaces: EventListener = () => {
    markLive();
    reconciler.wake();
    // Monitor changes missed while disconnected can fall outside the replay
    // window, so an open monitor popover re-reads on every (re)connect.
    void queryClient.invalidateQueries({ predicate: isSessionMonitorsQuery });
  };
  const handleCatalogChange: EventListener = event => {
    const payload = parseSessionCatalogEvent(event);
    if (!payload) return;
    markLive();
    reconciler.wake(payload.workspace_id);
    if (payload.kind === "layout") {
      const ids = new Set<string>();
      for (const query of queryClient.getQueryCache().findAll({ queryKey: sessionKeys.all })) {
        const key = query.queryKey;
        const data = query.state.data as SessionPayload | undefined;
        if (
          key[1] === "workspace" &&
          key[2] === payload.workspace_id &&
          key[3] === "detail" &&
          key.length === 5
        )
          ids.add(key[4] as string);
        if (key[1] === "by-id" && data?.workspace_id === payload.workspace_id)
          ids.add(key[2] as string);
      }
      for (const id of ids) sessionReads.wake(id, { ...payload, session_id: id });
      return;
    }
    sessionReads.wake(payload.session_id, payload);
    subagentLists.wake(payload.workspace_id, payload.session_id);
    // Only an open monitor popover observes this small read; elsewhere it is just
    // marked stale. An in-flight read may predate the change, so it is replaced.
    void invalidateSessionMonitorsQueries(queryClient, payload.workspace_id, payload.session_id);
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
    markLive();
    reconciler.wake(payload.workspace_id);
    sessionReads.wake(payload.session_id);
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
    markLive();
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
    clearStaleTimer();
    reconciler.close();
    subagentLists.close();
    sessionReads.close();
    for (const [type, listener] of listeners) source.removeEventListener(type, listener);
  };
  handleStreamError();
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
