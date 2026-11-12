import { useEffect, useEffectEvent, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import {
  buildWindowManagerStreamUrl,
  fetchWindowManagerSnapshot,
} from "../adapters/window-manager-api";
import {
  dispatchWindowManagerClientCommand,
  writeWindowManagerClientCommandFrame,
  type WindowManagerClientCommandExecutor,
} from "../lib/window-manager-client-command-frames";
import { parseWindowManagerStreamFrame } from "../lib/window-manager-stream-schema";
import {
  browserWindowManagerSocket,
  type WindowManagerSocket,
  type WindowManagerSocketFactory,
} from "../lib/window-manager-stream-socket";
import { reconcileWindowManagerSnapshot, windowManagerKeys } from "../lib/window-manager-query";
import type {
  LayoutRevision,
  WindowManagerAttachedClientView,
  WindowManagerClientView,
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
import {
  serializeWindowManagerClientContext,
  type WindowManagerClientContextInput,
} from "../lib/desktop-chrome-client-context";

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
  onClientCommand?: WindowManagerClientCommandExecutor;
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
    /** Reports a client-side stream failure and reconnects for a fresh fence snapshot. */
    recover: (cause: Error) => void;
  } | null>(null);
  // Consecutive client-side stream failures, kept across sockets so a frame that
  // fails on every reconnect backs off instead of spinning; a snapshot resets it.
  const recoveryAttemptRef = useRef(0);
  const contextRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The workspace/profile scope whose stream this hook has attempted. Before any
  // attempt the status is `idle` (nothing to report); after one — including while
  // the client re-registers — a stop is a real disconnect.
  const attemptedScopeRef = useRef<string | null>(null);
  const streamScope =
    workspaceId !== null && profileId !== null ? `${workspaceId}\u0000${profileId}` : null;
  const bindingKey =
    workspaceId !== null && profileId !== null && clientId !== null
      ? `${workspaceId}\u0000${profileId}\u0000${clientId}\u0000${registrationEpoch}`
      : null;
  const clientContextFrame =
    clientContext === undefined ? null : serializeWindowManagerClientContext(clientContext);
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
        bound.recover(
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
      publishStatus(
        streamScope !== null && attemptedScopeRef.current === streamScope ? "disconnected" : "idle"
      );
      return undefined;
    }

    if (bindingKey === null) {
      lifecycleStore.trigger.disabled();
      publishStatus("idle");
      return undefined;
    }

    let stopped = false;
    let receivedSnapshot = false;
    let clientRecoveryRequested = false;
    let refresh: Promise<void> | null = null;
    let refreshRetryTimer: ReturnType<typeof setTimeout> | null = null;
    let refreshRetryAttempt = 0;
    // Set once this socket is being replaced: the reconnect fence, not a refresh, clears the error.
    let recovering = false;
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
    attemptedScopeRef.current = streamScope;

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
          recoverStream(
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
      // Record the revision only once the subscriber took it: a throw leaves it
      // unrecorded, so the reconnect snapshot replays the same client view.
      publishClient(client);
      lifecycleStore.trigger.presentationObserved({ revision: client.presentationRevision });
    };

    const currentTopologyRevision = () =>
      queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot(workspaceId, profileId)
      )?.revision ?? -1;

    let refreshSnapshot: (minimumRevision: LayoutRevision) => void;
    const scheduleRefreshRetry = () => {
      if (stopped || recovering || refreshRetryTimer !== null) return;
      const delay = Math.min(8_000, 500 * 2 ** Math.min(refreshRetryAttempt, 4));
      refreshRetryAttempt += 1;
      refreshRetryTimer = setTimeout(() => {
        refreshRetryTimer = null;
        refreshSnapshot(pendingMinimumRevision);
      }, delay);
    };

    refreshSnapshot = (minimumRevision: LayoutRevision) => {
      pendingMinimumRevision = Math.max(pendingMinimumRevision, minimumRevision);
      if (recovering || refresh !== null || refreshRetryTimer !== null) return;
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
            try {
              applySnapshot(snapshot);
            } catch (cause) {
              // Applying, not reading, failed: reconnect for a fresh fence (#1543).
              retryNeeded = true;
              recoverStream(
                cause instanceof Error ? cause : new Error("Unable to apply the window layout.")
              );
              return;
            }
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

    // A client-side failure (bad or unapplied frame, throwing subscriber, failed
    // write or resync) has no daemon event to correct it: heartbeats keep the socket
    // alive at a revision we hold. Reconnect so a fresh fence snapshot clears the error.
    const recoverStream = (cause: Error) => {
      if (stopped) return;
      publishError(cause);
      recovering = true;
      if (refreshRetryTimer !== null) clearTimeout(refreshRetryTimer);
      refreshRetryTimer = null;
      const attempt = recoveryAttemptRef.current;
      recoveryAttemptRef.current = attempt + 1;
      liveness.restart(windowManagerReconnectDelay(attempt));
    };
    boundSocketRef.current = {
      bindingKey: activeBindingKey,
      ready: false,
      socket,
      recover: recoverStream,
    };

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
              recoverStream(
                sendCause instanceof Error
                  ? sendCause
                  : new Error("Unable to return the client operation result.")
              );
            }
          };
          dispatchWindowManagerClientCommand(
            frame.command,
            executeClientCommand,
            sendFrame,
            () => !stopped
          );
          return;
        }
        if (frame.type === "snapshot") {
          receivedSnapshot = true;
          lifecycleStore.trigger.snapshotObserved({ revision: frame.snapshot.revision });
          applySnapshot(frame.snapshot, true);
          if (frame.client !== null) applyClient(frame.client);
          publishStatus("connected");
          recoveryAttemptRef.current = 0;
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
        recoverStream(
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
    streamScope,
    workspaceId,
  ]);
}
