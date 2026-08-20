import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { useAtom } from "@xstate/store-react";
import { useEffect, useState } from "react";

import { useSession } from "@/systems/session";
import { useWorkspaceScopeMode } from "@/systems/workspace";

import type { OsShellHandle } from "../contexts/os-shell-context";
import { ClientCommandChannel } from "../lib/client-command-channel";
import {
  resolveLivePaletteClientContext,
  selectPaletteDestinationRoute,
} from "../lib/desktop-chrome-client-context";
import type {
  WindowManagerErrorPayload,
  WindowManagerRegisteredClientView,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import {
  windowManagerConfigOptions,
  windowManagerKeys,
  windowManagerSnapshotOptions,
} from "../lib/window-manager-query";
import { RoutingCoordinator, type OsRouterPort } from "../lib/routing-coordinator";
import { subscribeWorkspaceSwitchBarrier } from "../lib/workspace-switch-barrier";
import { WindowManagerRuntime } from "../runtime/window-manager-runtime";
import { selectFocusedSessionId } from "./use-focused-session-id";
import { useGlobalShortcutReconciliation } from "./use-global-shortcut-reconciliation";
import { useWindowManagerClient } from "./use-window-manager-client";
import { useWindowManagerStream } from "./use-window-manager-stream";
import { useWindowPaletteIntent } from "./use-window-manager-store";

export interface DesktopChromeModel {
  shell: OsShellHandle;
  query: UseQueryResult<WindowManagerSnapshot, Error>;
  /** The attachment this shell speaks for; the palette projection needs its context. */
  client: WindowManagerRegisteredClientView | null;
  /** Executes a `client_op` the daemon pushed over the window-manager channel. */
  clientCommandChannel: ClientCommandChannel;
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
 *
 * This hook creates the OsShellHandle that DesktopShell provides. It must read
 * that projection atom directly — useDesktop/useOsShell require the provider
 * this hook feeds (BUG-20260813).
 */
export function useDesktopChrome(activeWorkspaceId: string | null): DesktopChromeModel {
  const router = useRouter();
  const queryClient = useQueryClient();
  const scope = useWorkspaceScopeMode();
  const paletteIntent = useWindowPaletteIntent();
  const query = useQuery(windowManagerSnapshotOptions(activeWorkspaceId ?? ""));
  const client = useWindowManagerClient(activeWorkspaceId);
  // Scoped to the active workspace: a rebind stored in the workspace overlay has
  // to reach the shell's own keymap, or the chord would not dispatch until a
  // reload (US-022.AC-3).
  const configQuery = useQuery(windowManagerConfigOptions(activeWorkspaceId, client.clientId));
  const globalShortcuts = useGlobalShortcutReconciliation(configQuery.data?.globalShortcuts);
  const [manager] = useState(() => new WindowManagerRuntime(queryClient));
  // The seam is built deeper in the tree (it needs the shell's own handlers), so
  // the stream reaches it through a ref rather than the shell reaching upward.
  const [clientCommandChannel] = useState(() => new ClientCommandChannel());
  const [shell] = useState<OsShellHandle>(() => {
    const port: OsRouterPort = {
      navigate: route => navigateTo(router, route, false),
      replace: route => navigateTo(router, route, true),
    };
    return {
      projection: manager.projectionAtom,
      manager,
      coordinator: new RoutingCoordinator(manager, port),
    };
  });
  const focusedSessionId = useAtom(manager.projectionAtom, selectFocusedSessionId);
  const destinationRoute = useAtom(manager.projectionAtom, state =>
    selectPaletteDestinationRoute(state, paletteIntent?.windowId)
  );
  const focusedSession = useSession(focusedSessionId ?? "", activeWorkspaceId);

  useEffect(() => {
    manager.start();
    return () => manager.stop();
  }, [manager]);

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

  // Bind first: workspace selection and registration can settle in the same
  // commit, and setClient intentionally rejects a client for an unbound runtime.
  useEffect(() => {
    manager.setClient(client.client);
    manager.setLoadError(client.error);
    shell.coordinator.reportAuthoritativeState();
  }, [activeWorkspaceId, client.client, client.error, manager, shell]);

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
    clientContext: resolveLivePaletteClientContext({
      scope,
      focusedSessionState: focusedSession.data?.state,
      registeredWorkspaceTrusted: client.client?.paletteContext.workspaceTrusted,
      destinationRoute,
      globalShortcuts: globalShortcuts.registrations,
    }),
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
      if (view.globalShortcuts.length > 0) {
        void queryClient.invalidateQueries({
          queryKey: windowManagerKeys.config(activeWorkspaceId, client.clientId),
          exact: true,
        });
      }
    },
    onClientCommand: command => {
      return clientCommandChannel.execute(command.op, command.payload);
    },
    onClientInvalidated: client.reregister,
    onError: error => manager.setLoadError(streamError(error)),
  });

  return {
    shell,
    query,
    client: client.client,
    clientCommandChannel,
  };
}
