import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

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

export interface WindowManagerSocket {
  close: () => void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type WindowManagerSocketFactory = (url: string) => WindowManagerSocket;

function browserWindowManagerSocket(url: string): WindowManagerSocket {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return new WebSocket(`${protocol}//${window.location.host}${url}`);
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
  const [reconnectEpoch, setReconnectEpoch] = useState(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttempt = useRef(0);
  const latestBindingKey = useRef<string | null>(null);
  const latestTopologyRevision = useRef(afterRevision);
  const latestPresentationRevision = useRef(0);
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
    if (latestBindingKey.current !== bindingKey) {
      latestBindingKey.current = bindingKey;
      latestTopologyRevision.current = afterRevision;
      latestPresentationRevision.current = boundClient?.presentationRevision ?? 0;
      reconnectAttempt.current = 0;
      return;
    }
    latestTopologyRevision.current = Math.max(latestTopologyRevision.current, afterRevision);
    if (boundClient !== null) {
      latestPresentationRevision.current = Math.max(
        latestPresentationRevision.current,
        boundClient.presentationRevision
      );
    }
  }, [afterRevision, bindingKey, clientId, currentClient, workspaceId]);

  useEffect(() => {
    if (
      !enabled ||
      workspaceId === null ||
      clientId === null ||
      typeof window === "undefined" ||
      (!socketFactory && typeof WebSocket === "undefined")
    ) {
      reconnectAttempt.current = 0;
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
    const attempt = reconnectAttempt.current;
    publishStatus(attempt === 0 ? "connecting" : "reconnecting");
    const cachedAtOpen = queryClient.getQueryData<WindowManagerSnapshot>(
      windowManagerKeys.snapshot(workspaceId)
    );
    const topologyFence = Math.max(latestTopologyRevision.current, cachedAtOpen?.revision ?? 0);
    const socket = (socketFactory ?? browserWindowManagerSocket)(
      buildWindowManagerStreamUrl(workspaceId, clientId, topologyFence)
    );

    const applySnapshot = (snapshot: WindowManagerSnapshot) => {
      if (snapshot.workspaceId !== workspaceId) return;
      latestTopologyRevision.current = Math.max(latestTopologyRevision.current, snapshot.revision);
      const key = windowManagerKeys.snapshot(workspaceId);
      queryClient.setQueryData<WindowManagerSnapshot>(key, current =>
        reconcileWindowManagerSnapshot(current, snapshot)
      );
      publishSnapshot(snapshot);
    };

    const applyClient = (client: WindowManagerClientView) => {
      if (client.workspaceId !== workspaceId || client.clientId !== clientId) return;
      if (client.presentationRevision <= latestPresentationRevision.current) return;
      latestPresentationRevision.current = client.presentationRevision;
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
          publishError(frame.error);
          return;
        }
        if (frame.workspaceId !== workspaceId) return;
        if (frame.type === "snapshot") {
          receivedSnapshot = true;
          reconnectAttempt.current = 0;
          applySnapshot(frame.snapshot);
          if (frame.client !== null) applyClient(frame.client);
          publishStatus("connected");
          return;
        }
        if (frame.type === "client") {
          applyClient(frame.client);
          return;
        }
        latestTopologyRevision.current = Math.max(latestTopologyRevision.current, frame.revision);
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
      if (stopped) return;
      publishStatus("reconnecting");
      const closingAttempt = reconnectAttempt.current;
      const delay = Math.min(8_000, 500 * 2 ** Math.min(closingAttempt, 4));
      reconnectTimer.current = setTimeout(() => {
        reconnectTimer.current = null;
        reconnectAttempt.current = closingAttempt + 1;
        setReconnectEpoch(current => current + 1);
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
      if (reconnectTimer.current !== null) {
        clearTimeout(reconnectTimer.current);
        reconnectTimer.current = null;
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
    socketFactory,
    workspaceId,
  ]);
}
