import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import type { OsShellHandle } from "../contexts/os-shell-context";
import type { WindowManagerErrorPayload, WindowManagerSnapshot } from "../lib/window-manager-types";
import {
  windowManagerConfigOptions,
  windowManagerSnapshotOptions,
} from "../lib/window-manager-query";
import { RoutingCoordinator, type OsRouterPort } from "../lib/routing-coordinator";
import { subscribeWorkspaceSwitchBarrier } from "../lib/workspace-switch-barrier";
import { WindowManagerRuntime } from "../runtime/window-manager-runtime";
import { useWindowManagerClient } from "./use-window-manager-client";
import { useWindowManagerStream } from "./use-window-manager-stream";

export interface DesktopChromeModel {
  shell: OsShellHandle;
  query: UseQueryResult<WindowManagerSnapshot, Error>;
}

function navigateTo(
  router: ReturnType<typeof useRouter>,
  route: { pathname: string; search: Record<string, unknown> },
  replace: boolean
): void {
  const href = `${route.pathname}${router.options.stringifySearch(route.search)}`;
  if (replace) router.history.replace(href);
  else router.history.push(href);
}

function streamError(error: Error | WindowManagerErrorPayload): Error {
  return error instanceof Error ? error : new Error(error.error);
}

/**
 * Owns the workspace-scoped Query, stable client registration, fenced stream,
 * and the transient projection runtime. Query remains the only snapshot owner.
 */
export function useDesktopChrome(activeWorkspaceId: string | null): DesktopChromeModel {
  const router = useRouter();
  const queryClient = useQueryClient();
  const query = useQuery(windowManagerSnapshotOptions(activeWorkspaceId ?? ""));
  const configQuery = useQuery(windowManagerConfigOptions());
  const [manager] = useState(() => new WindowManagerRuntime(queryClient));
  const [shell] = useState<OsShellHandle>(() => {
    const port: OsRouterPort = {
      navigate: route => navigateTo(router, route, false),
      replace: route => navigateTo(router, route, true),
    };
    return {
      store: manager,
      manager,
      coordinator: new RoutingCoordinator(manager, port),
    };
  });

  useEffect(() => {
    manager.start();
    return () => manager.stop();
  }, [manager]);

  const client = useWindowManagerClient(
    activeWorkspaceId,
    view => {
      manager.setClient(view);
      shell.coordinator.reportAuthoritativeState();
    },
    error => manager.setLoadError(error)
  );

  useEffect(() => subscribeWorkspaceSwitchBarrier(shell.coordinator), [shell]);

  useEffect(() => {
    if (activeWorkspaceId === null) {
      manager.unbind();
      return undefined;
    }
    manager.bind({ workspaceId: activeWorkspaceId, clientId: client.clientId });
    shell.coordinator.beginWorkspaceSwitch();
    return () => manager.unbind();
  }, [activeWorkspaceId, client.clientId, manager, shell]);

  useEffect(() => {
    if (client.status === "registered" && query.data && configQuery.data) {
      shell.coordinator.completeHydration();
    }
  }, [client.status, configQuery.data, query.data, shell]);

  useWindowManagerStream({
    workspaceId: activeWorkspaceId,
    clientId: client.clientId,
    registrationEpoch: client.registrationEpoch,
    currentClient: client.client,
    enabled: client.status === "registered",
    afterRevision: query.data?.revision ?? 0,
    onStatusChange: status => manager.setConnectionStatus(status),
    onSnapshot: () => {
      manager.setLoadError(null);
      shell.coordinator.reportAuthoritativeState();
    },
    onClient: view => {
      manager.setClient(view);
      shell.coordinator.reportAuthoritativeState();
    },
    onClientInvalidated: client.reregister,
    onError: error => manager.setLoadError(streamError(error)),
  });

  return {
    shell,
    query,
  };
}
