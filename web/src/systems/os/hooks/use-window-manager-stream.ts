import { useEffect, useEffectEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { createStreamWebSocket } from "@/lib/ticketed-web-socket";

import {
  buildWindowManagerStreamUrl,
  fetchWindowManagerSnapshot,
} from "../adapters/window-manager-api";
import { parseWindowManagerStreamFrame } from "../lib/window-manager-stream-schema";
import { reconcileWindowManagerSnapshot, windowManagerKeys } from "../lib/window-manager-query";
import type {
  LayoutRevision,
  WindowManagerClientView,
  WindowManagerConnectionStatus,
  WindowManagerErrorPayload,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import { windowManagerStreamLogic } from "./window-manager-stream-store";
import { workspaceKeys } from "@/systems/workspace";

export interface WindowManagerSocket {
  close: () => void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type WindowManagerSocketFactory = (url: string) => WindowManagerSocket;

function browserWindowManagerSocket(url: string): WindowManagerSocket {
  return createStreamWebSocket(url);
}

export interface UseWindowManagerStreamOptions {
  workspaceId: string | null;
  clientId: string | null;
  registrationEpoch: number;
  currentClient: WindowManagerClientView | null;
  enabled: boolean;
  afterRevision: LayoutRevision;
  socketFactory?: WindowManagerSocketFactory;
  onStatusChange: (status: WindowManagerConnectionStatus) => void;
  onSnapshot: (snapshot: WindowManagerSnapshot) => void;
  onClient: (client: WindowManagerClientView) => void;
  onClientInvalidated: () => void;
  onError: (error: Error | WindowManagerErrorPayload) => void;
}

export function useWindowManagerStream({
  workspaceId,
  clientId,
  registrationEpoch,
  currentClient,
  enabled,
  afterRevision,
  socketFactory,
  onStatusChange,
  onSnapshot,
  onClient,
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
  const publishClientInvalidated = useEffectEvent(onClientInvalidated);
  const publishError = useEffectEvent(onError);
  const bindingKey =
    workspaceId !== null && clientId !== null
      ? `${workspaceId}\u0000${clientId}\u0000${registrationEpoch}`
      : null;

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
    if (
      !enabled ||
      workspaceId === null ||
      clientId === null ||
      typeof window === "undefined" ||
      (!socketFactory && typeof WebSocket === "undefined")
    ) {
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
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    const refreshController = new AbortController();
    const cachedAtOpen = queryClient.getQueryData<WindowManagerSnapshot>(
      windowManagerKeys.snapshot(workspaceId)
    );
    const topologyFence = Math.max(
      lifecycleStore.getSnapshot().context.topologyRevision,
      cachedAtOpen?.revision ?? 0
    );
    const socket = (socketFactory ?? browserWindowManagerSocket)(
      buildWindowManagerStreamUrl(workspaceId, clientId, topologyFence)
    );

    const applySnapshot = (snapshot: WindowManagerSnapshot) => {
      if (snapshot.workspaceId !== workspaceId) return;
      lifecycleStore.trigger.topologyObserved({ revision: snapshot.revision });
      const key = windowManagerKeys.snapshot(workspaceId);
      queryClient.setQueryData<WindowManagerSnapshot>(key, current =>
        reconcileWindowManagerSnapshot(current, snapshot)
      );
      publishSnapshot(snapshot);
    };

    const applyClient = (client: WindowManagerClientView) => {
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
      queryClient.getQueryData<WindowManagerSnapshot>(windowManagerKeys.snapshot(workspaceId))
        ?.revision ?? -1;

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
          const snapshot = await fetchWindowManagerSnapshot(workspaceId, refreshController.signal);
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

    socket.onopen = () => {
      if (!stopped) publishStatus("connecting");
    };
    socket.onmessage = event => {
      if (stopped || typeof event.data !== "string") return;
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
        if (frame.type === "snapshot") {
          receivedSnapshot = true;
          lifecycleStore.trigger.snapshotObserved({ revision: frame.snapshot.revision });
          applySnapshot(frame.snapshot);
          if (frame.client !== null) applyClient(frame.client);
          publishStatus("connected");
          return;
        }
        if (frame.type === "client") {
          applyClient(frame.client);
          return;
        }
        lifecycleStore.trigger.topologyObserved({ revision: frame.revision });
        const cached = queryClient.getQueryData<WindowManagerSnapshot>(
          windowManagerKeys.snapshot(workspaceId)
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
      if (stopped || reconnectTimer !== null) return;
      publishStatus("reconnecting");
      const closingAttempt = lifecycleStore.getSnapshot().context.reconnectAttempt;
      const delay = Math.min(8_000, 500 * 2 ** Math.min(closingAttempt, 4));
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        lifecycleStore.trigger.reconnectElapsed({ attempt: closingAttempt });
      }, delay);
    };

    return () => {
      stopped = true;
      refreshController.abort();
      socket.onopen = null;
      socket.onmessage = null;
      socket.onerror = null;
      socket.onclose = null;
      socket.close();
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (refreshRetryTimer !== null) {
        clearTimeout(refreshRetryTimer);
        refreshRetryTimer = null;
      }
      if (receivedSnapshot) publishStatus("disconnected");
    };
  }, [
    clientId,
    enabled,
    queryClient,
    reconnectEpoch,
    registrationEpoch,
    lifecycleStore,
    socketFactory,
    workspaceId,
  ]);
}
