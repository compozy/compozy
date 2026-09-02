import { useEffect, useEffectEvent, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { createStreamWebSocket } from "@/lib/ticketed-web-socket";

import {
  buildWindowManagerStreamUrl,
  fetchWindowManagerSnapshot,
} from "../adapters/window-manager-api";
import { writeWindowManagerClientCommandFrame } from "../lib/window-manager-client-command-frames";
import { parseWindowManagerStreamFrame } from "../lib/window-manager-stream-schema";
import { reconcileWindowManagerSnapshot, windowManagerKeys } from "../lib/window-manager-query";
import type {
  LayoutRevision,
  WindowManagerAttachedClientView,
  WindowManagerClientView,
  WindowManagerClientCommand,
  WindowManagerConnectionStatus,
  WindowManagerErrorPayload,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import {
  createStreamLiveness,
  windowManagerReconnectDelay,
} from "./window-manager-stream-liveness";
import { windowManagerStreamLogic } from "./window-manager-stream-store";
import { workspaceKeys } from "@/systems/workspace";
import type { GlobalShortcutRegistrationWire } from "../lib/desktop-shell-bridge";

export interface WindowManagerSocket {
  close: () => void;
  send: (data: string) => void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type WindowManagerSocketFactory = (url: string) => WindowManagerSocket;

export interface WindowManagerClientContextInput {
  scopeGlobal: boolean;
  focusedSessionState: string | null;
  workspaceTrusted: boolean;
  destinationIntent: {
    pathname: string;
    search: Readonly<Record<string, unknown>>;
  } | null;
  globalShortcuts: readonly GlobalShortcutRegistrationWire[];
}

function browserWindowManagerSocket(url: string): WindowManagerSocket {
  return createStreamWebSocket(url);
}

export {
  WINDOW_MANAGER_HEARTBEAT_INTERVAL_MS,
  WINDOW_MANAGER_STREAM_STALL_MS,
  windowManagerReconnectDelay,
} from "./window-manager-stream-liveness";

export interface UseWindowManagerStreamOptions {
  workspaceId: string | null;
  /** The profile whose desks this stream carries; a switch reconnects (US-026). */
  profileId: string | null;
  clientId: string | null;
  registrationEpoch: number;
  currentClient: WindowManagerClientView | null;
  clientContext?: WindowManagerClientContextInput;
  enabled: boolean;
  afterRevision: LayoutRevision;
  socketFactory?: WindowManagerSocketFactory;
  onStatusChange: (status: WindowManagerConnectionStatus) => void;
  onSnapshot: (snapshot: WindowManagerSnapshot) => void;
  onClient: (client: WindowManagerAttachedClientView) => void;
  onClientCommand?: (command: WindowManagerClientCommand) => unknown | Promise<unknown>;
  onClientInvalidated: () => void;
  onError: (error: Error | WindowManagerErrorPayload) => void;
}

export function useWindowManagerStream({
  workspaceId,
  profileId,
  clientId,
  registrationEpoch,
  currentClient,
  clientContext,
  enabled,
  afterRevision,
  socketFactory,
  onStatusChange,
  onSnapshot,
  onClient,
  onClientCommand,
  onClientInvalidated,
  onError,
}: UseWindowManagerStreamOptions): void {
  const queryClient = useQueryClient();
  const lifecycleStore = useStore(windowManagerStreamLogic, {
    topologyRevision: afterRevision,
  });
  const reconnectEpoch = useSelector(lifecycleStore, snapshot => snapshot.context.reconnectEpoch);
  const publishStatus = useEffectEvent(onStatusChange);
  const publishSnapshot = useEffectEvent(onSnapshot);
  const publishClient = useEffectEvent(onClient);
  const executeClientCommand = useEffectEvent(
    onClientCommand ??
      (command => {
        throw new Error(`Unsupported client operation: ${command.op}`);
      })
  );
  const publishClientInvalidated = useEffectEvent(onClientInvalidated);
  const publishError = useEffectEvent(onError);
  const boundSocketRef = useRef<{
    bindingKey: string;
    ready: boolean;
    socket: WindowManagerSocket;
  } | null>(null);
  const contextRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const bindingKey =
    workspaceId !== null && profileId !== null && clientId !== null
      ? `${workspaceId}\u0000${profileId}\u0000${clientId}\u0000${registrationEpoch}`
      : null;
  const clientContextFrame =
    clientContext === undefined
      ? null
      : JSON.stringify({
          type: "client_context",
          context: {
            scope_global: clientContext.scopeGlobal,
            ...(clientContext.focusedSessionState === null
              ? {}
              : { focused_session_state: clientContext.focusedSessionState }),
            workspace_trusted: clientContext.workspaceTrusted,
            ...(clientContext.destinationIntent === null
              ? {}
              : { destination_intent: clientContext.destinationIntent }),
            global_shortcuts: clientContext.globalShortcuts,
          },
        });
  const scheduleContextRefresh = useEffectEvent(() => {
    if (contextRefreshTimerRef.current !== null) {
      clearTimeout(contextRefreshTimerRef.current);
    }
    contextRefreshTimerRef.current = setTimeout(() => {
      contextRefreshTimerRef.current = null;
      const bound = boundSocketRef.current;
      if (
        bound === null ||
        !bound.ready ||
        bindingKey === null ||
        bound.bindingKey !== bindingKey ||
        clientContextFrame === null
      ) {
        return;
      }
      try {
        bound.socket.send(clientContextFrame);
      } catch (cause) {
        publishError(
          cause instanceof Error ? cause : new Error("Unable to refresh the client context.")
        );
      }
    }, 75);
  });

  useEffect(() => {
    const boundClient =
      currentClient?.workspaceId === workspaceId && currentClient.clientId === clientId
        ? currentClient
        : null;
    lifecycleStore.trigger.bindingObserved({
      bindingKey,
      presentationRevision: boundClient?.presentationRevision ?? 0,
      topologyRevision: afterRevision,
    });
  }, [afterRevision, bindingKey, clientId, currentClient, lifecycleStore, workspaceId]);

  useEffect(() => {
    if (clientContextFrame === null || bindingKey === null || !enabled) return undefined;
    scheduleContextRefresh();
    return () => {
      if (contextRefreshTimerRef.current !== null) {
        clearTimeout(contextRefreshTimerRef.current);
        contextRefreshTimerRef.current = null;
      }
    };
  }, [bindingKey, clientContextFrame, enabled]);

  useEffect(() => {
    if (
      !enabled ||
      workspaceId === null ||
      profileId === null ||
      clientId === null ||
      typeof window === "undefined" ||
      (!socketFactory && typeof WebSocket === "undefined")
    ) {
      lifecycleStore.trigger.disabled();
      publishStatus("disconnected");
      return undefined;
    }

    if (bindingKey === null) {
      lifecycleStore.trigger.disabled();
      publishStatus("disconnected");
      return undefined;
    }

    let stopped = false;
    let receivedSnapshot = false;
    let clientRecoveryRequested = false;
    let refresh: Promise<void> | null = null;
    let refreshRetryTimer: ReturnType<typeof setTimeout> | null = null;
    let refreshRetryAttempt = 0;
    let pendingMinimumRevision = -1;
    const refreshController = new AbortController();
    const cachedAtOpen = queryClient.getQueryData<WindowManagerSnapshot>(
      windowManagerKeys.snapshot(workspaceId, profileId)
    );
    const topologyFence = Math.max(
      lifecycleStore.getSnapshot().context.topologyRevision,
      cachedAtOpen?.revision ?? 0
    );
    const socket = (socketFactory ?? browserWindowManagerSocket)(
      buildWindowManagerStreamUrl(workspaceId, profileId, clientId, topologyFence)
    );
    const activeBindingKey = bindingKey;
    boundSocketRef.current = { bindingKey: activeBindingKey, ready: false, socket };

    /**
     * Cache the snapshot. Events and refreshes only ever move the cache forward;
     * an authoritative read (the stream fence, or a re-read the daemon itself
     * announced as behind us) replaces the cache even when its revision is not
     * ahead, because the daemon restarted from a discarded, replaced, or migrated
     * arrangement and a client that keeps the old content would show it forever.
     */
    const applySnapshot = (snapshot: WindowManagerSnapshot, authoritative = false) => {
      if (snapshot.workspaceId !== workspaceId) return;
      const key = windowManagerKeys.snapshot(workspaceId, profileId);
      const cached = queryClient.getQueryData<WindowManagerSnapshot>(key);
      if (authoritative && cached !== undefined && snapshot.revision <= cached.revision) {
        lifecycleStore.trigger.snapshotObserved({ revision: snapshot.revision });
        queryClient.setQueryData<WindowManagerSnapshot>(key, snapshot);
      } else {
        lifecycleStore.trigger.topologyObserved({ revision: snapshot.revision });
        queryClient.setQueryData<WindowManagerSnapshot>(key, current =>
          reconcileWindowManagerSnapshot(current, snapshot)
        );
      }
      publishSnapshot(snapshot);
    };

    let resync: Promise<void> | null = null;
    /** The daemon announced a revision behind the cache: re-read and accept its truth. */
    const resyncSnapshot = () => {
      if (resync !== null) return;
      resync = fetchWindowManagerSnapshot(workspaceId, profileId, refreshController.signal)
        .then(snapshot => {
          if (stopped) return;
          applySnapshot(snapshot, true);
        })
        .catch(cause => {
          if (stopped) return;
          publishError(
            cause instanceof Error ? cause : new Error("Unable to re-read the window layout.")
          );
        })
        .finally(() => {
          resync = null;
        });
    };

    const applyClient = (client: WindowManagerAttachedClientView) => {
      if (client.workspaceId !== workspaceId || client.clientId !== clientId) return;
      if (
        client.presentationRevision <= lifecycleStore.getSnapshot().context.presentationRevision
      ) {
        return;
      }
      lifecycleStore.trigger.presentationObserved({ revision: client.presentationRevision });
      publishClient(client);
    };

    const currentTopologyRevision = () =>
      queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot(workspaceId, profileId)
      )?.revision ?? -1;

    let refreshSnapshot: (minimumRevision: LayoutRevision) => void;
    const scheduleRefreshRetry = () => {
      if (stopped || refreshRetryTimer !== null) return;
      const delay = Math.min(8_000, 500 * 2 ** Math.min(refreshRetryAttempt, 4));
      refreshRetryAttempt += 1;
      refreshRetryTimer = setTimeout(() => {
        refreshRetryTimer = null;
        refreshSnapshot(pendingMinimumRevision);
      }, delay);
    };

    refreshSnapshot = (minimumRevision: LayoutRevision) => {
      pendingMinimumRevision = Math.max(pendingMinimumRevision, minimumRevision);
      if (refresh !== null || refreshRetryTimer !== null) return;
      let retryNeeded = false;
      refresh = (async () => {
        while (!stopped && currentTopologyRevision() < pendingMinimumRevision) {
          const before = currentTopologyRevision();
          const snapshot = await fetchWindowManagerSnapshot(
            workspaceId,
            profileId,
            refreshController.signal
          );
          if (stopped) return;
          if (snapshot.revision > before) {
            applySnapshot(snapshot);
            refreshRetryAttempt = 0;
            continue;
          }
          retryNeeded = true;
          return;
        }
      })()
        .catch(cause => {
          if (stopped) return;
          retryNeeded = true;
          publishError(
            cause instanceof Error ? cause : new Error("Unable to refresh the window layout.")
          );
        })
        .finally(() => {
          refresh = null;
          if (stopped || currentTopologyRevision() >= pendingMinimumRevision) {
            refreshRetryAttempt = 0;
            return;
          }
          if (retryNeeded) {
            scheduleRefreshRetry();
          } else {
            refreshSnapshot(pendingMinimumRevision);
          }
        });
    };

    const recoverMissingClient = () => {
      if (clientRecoveryRequested) return;
      clientRecoveryRequested = true;
      publishClientInvalidated();
    };

    // One reconnect per socket: the first trigger (close, stall, wake) wins.
    const liveness = createStreamLiveness({
      reconnectAttempt: () => lifecycleStore.getSnapshot().context.reconnectAttempt,
      onReconnect: attempt => lifecycleStore.trigger.reconnectElapsed({ attempt }),
      onStatus: status => {
        if (!stopped) publishStatus(status);
      },
      closeSocket: () => {
        socket.onclose = null;
        socket.onerror = null;
        socket.close();
      },
    });

    socket.onopen = () => {
      if (stopped) return;
      const bound = boundSocketRef.current;
      if (bound?.socket === socket) bound.ready = true;
      liveness.noteFrame();
      scheduleContextRefresh();
      publishStatus("connecting");
    };
    socket.onmessage = event => {
      if (stopped || typeof event.data !== "string") return;
      liveness.noteFrame();
      try {
        const frame = parseWindowManagerStreamFrame(JSON.parse(event.data) as unknown);
        if (frame.type === "error") {
          if (frame.error.code === "window_manager_client_not_found") {
            recoverMissingClient();
          }
          if (frame.error.code === "window_manager_workspace_not_found") {
            void queryClient.invalidateQueries({ queryKey: workspaceKeys.list(), exact: true });
          }
          publishError(frame.error);
          return;
        }
        if (frame.workspaceId !== workspaceId) return;
        if (frame.type === "client_command") {
          const sendFrame = (
            outbound: Parameters<typeof writeWindowManagerClientCommandFrame>[1]
          ) => {
            try {
              writeWindowManagerClientCommandFrame(data => socket.send(data), outbound);
            } catch (sendCause) {
              publishError(
                sendCause instanceof Error
                  ? sendCause
                  : new Error("Unable to return the client operation result.")
              );
            }
          };
          sendFrame({ type: "client_command_ack", command_id: frame.command.commandId });
          void Promise.resolve()
            .then(() => executeClientCommand(frame.command))
            .then(result => {
              if (stopped) return;
              sendFrame({
                type: "client_command_result",
                command_id: frame.command.commandId,
                ...(result === undefined ? {} : { result }),
              });
            })
            .catch(cause => {
              if (stopped) return;
              const error =
                cause instanceof Error ? cause : new Error("The client operation failed.");
              sendFrame({
                type: "client_command_result",
                command_id: frame.command.commandId,
                error: error.message,
              });
            });
          return;
        }
        if (frame.type === "snapshot") {
          receivedSnapshot = true;
          lifecycleStore.trigger.snapshotObserved({ revision: frame.snapshot.revision });
          applySnapshot(frame.snapshot, true);
          if (frame.client !== null) applyClient(frame.client);
          publishStatus("connected");
          return;
        }
        if (frame.type === "client") {
          applyClient(frame.client);
          return;
        }
        if (frame.type === "heartbeat") {
          const cachedRevision = currentTopologyRevision();
          if (frame.revision > cachedRevision) refreshSnapshot(frame.revision);
          else if (frame.revision < cachedRevision) resyncSnapshot();
          return;
        }
        lifecycleStore.trigger.topologyObserved({ revision: frame.revision });
        const cached = queryClient.getQueryData<WindowManagerSnapshot>(
          windowManagerKeys.snapshot(workspaceId, profileId)
        );
        if (frame.revision > (cached?.revision ?? -1)) refreshSnapshot(frame.revision);
      } catch (cause) {
        publishError(
          cause instanceof Error ? cause : new Error("The window-manager stream was invalid.")
        );
      }
    };
    socket.onerror = () => {
      if (stopped) return;
      publishStatus("reconnecting");
    };
    socket.onclose = () => {
      if (stopped) return;
      liveness.scheduleReconnect(
        windowManagerReconnectDelay(lifecycleStore.getSnapshot().context.reconnectAttempt)
      );
    };

    return () => {
      stopped = true;
      liveness.dispose();
      if (boundSocketRef.current?.socket === socket) boundSocketRef.current = null;
      if (contextRefreshTimerRef.current !== null) {
        clearTimeout(contextRefreshTimerRef.current);
        contextRefreshTimerRef.current = null;
      }
      refreshController.abort();
      socket.onopen = null;
      socket.onmessage = null;
      socket.onerror = null;
      socket.onclose = null;
      socket.close();
      if (refreshRetryTimer !== null) {
        clearTimeout(refreshRetryTimer);
        refreshRetryTimer = null;
      }
      if (receivedSnapshot) publishStatus("disconnected");
    };
  }, [
    bindingKey,
    clientId,
    enabled,
    profileId,
    queryClient,
    reconnectEpoch,
    registrationEpoch,
    lifecycleStore,
    socketFactory,
    workspaceId,
  ]);
}
