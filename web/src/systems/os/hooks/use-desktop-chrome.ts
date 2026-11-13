import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { useAtom } from "@xstate/store-react";
import { useEffect, useEffectEvent, useRef, useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";
import { useSession } from "@/systems/session";
import { isReservedDesktopPartition, useBreadthMode } from "@/systems/workspace";

import type { OsShellHandle } from "../contexts/os-shell-context";
import { ClientCommandChannel } from "../lib/client-command-channel";
import { TerminalWindowClose } from "../lib/terminal-window-close";
import { composeCloseGuards, UnsavedWindowClose } from "../lib/unsaved-window-close";
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
import { RoutingCoordinator } from "../lib/routing-coordinator";
import { connectRouterEntries, RouterEntryBridge } from "../lib/router-entry-bridge";
import { RoutingEntryLedger } from "../lib/routing-entry-ledger";
import { subscribeWorkspaceSwitchBarrier } from "../lib/workspace-switch-barrier";
import { WindowManagerRuntime } from "../runtime/window-manager-runtime";
import { selectFocusedSessionId } from "./use-focused-session-id";
import { useGlobalShortcutReconciliation } from "./use-global-shortcut-reconciliation";
import { useProfileSwitchWindowSweep } from "./use-profile-switch-window-sweep";
import { useWindowScopePruning } from "./use-window-scope-pruning";
import { clientRestore, useWindowManagerClient } from "./use-window-manager-client";
import { useWindowManagerStream } from "./use-window-manager-stream";
import { useWindowPaletteIntent } from "./use-window-manager-store";

export interface DesktopChromeModel {
  terminalClose: TerminalWindowClose;
  /** Holds operator closes of windows whose editors have unsaved edits (#1653). */
  unsavedClose: UnsavedWindowClose;
  shell: OsShellHandle;
  query: UseQueryResult<WindowManagerSnapshot, Error>;
  /** The attachment this shell speaks for; the palette projection needs its context. */
  client: WindowManagerRegisteredClientView | null;
  /** Executes a `client_op` the daemon pushed over the window-manager channel. */
  clientCommandChannel: ClientCommandChannel;
  /** The profile whose desk this render presents; the runtime catches up in effects. */
  profileId: string;
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
export function useDesktopChrome(
  activeWorkspaceId: string | null,
  terminalWorkspaceId: string | null = activeWorkspaceId
): DesktopChromeModel {
  const router = useRouter();
  const queryClient = useQueryClient();
  const breadthMode = useBreadthMode();
  const paletteIntent = useWindowPaletteIntent();
  // Desks belong to the profile this client acts as. Under the aggregate that is
  // the destination profile, so the desk and the work opened on it stay in the
  // same partition rather than drifting apart (ADR-005, US-026).
  const profileId = useProfileReadScope().destination;
  const query = useQuery(windowManagerSnapshotOptions(activeWorkspaceId ?? "", profileId));
  const client = useWindowManagerClient(activeWorkspaceId, profileId);
  useWindowScopePruning(profileId, query);
  // Scoped to the active workspace: a rebind stored in the workspace overlay has
  // to reach the shell's own keymap, or the chord would not dispatch until a
  // reload (US-022.AC-3). Reserved partitions (`global`, `user`) have no
  // workspace overlay.
  const configWorkspaceId = isReservedDesktopPartition(activeWorkspaceId)
    ? null
    : activeWorkspaceId;
  // The read names this client, so it waits until the client is attached to
  // the workspace it is about to present (a switch re-registers it).
  const configQuery = useQuery({
    ...windowManagerConfigOptions(configWorkspaceId, client.clientId, profileId),
    enabled: client.status === "registered",
  });
  const globalShortcuts = useGlobalShortcutReconciliation(configQuery.data?.globalShortcuts);
  const [manager] = useState(() => new WindowManagerRuntime(queryClient));
  const [terminalClose] = useState(() => new TerminalWindowClose());
  const [unsavedClose] = useState(() => new UnsavedWindowClose());
  useEffect(() => {
    // Unsaved edits ask first: keeping them must not leave a terminal already terminated.
    manager.setCloseGuard(
      composeCloseGuards(unsavedClose.guard, terminalClose.guard(terminalWorkspaceId, queryClient)),
      unsavedClose.settled
    );
    return () => {
      unsavedClose.cancel();
      terminalClose.cancel();
      manager.setCloseGuard(null);
    };
  }, [manager, terminalClose, unsavedClose, terminalWorkspaceId, profileId, queryClient]);
  // The seam is built deeper in the tree (it needs the shell's own handlers), so
  // the stream reaches it through a ref rather than the shell reaching upward.
  const [clientCommandChannel] = useState(() => new ClientCommandChannel());
  const [entries] = useState(() => new RoutingEntryLedger());
  const [bridge] = useState(
    () =>
      new RouterEntryBridge(
        router.history,
        route => `${route.pathname}${router.options.stringifySearch(route.search)}`
      )
  );
  const [shell] = useState<OsShellHandle>(() => ({
    projection: manager.projectionAtom,
    manager,
    coordinator: new RoutingCoordinator(manager, bridge.port, entries),
  }));
  // Every history change reaches the ledger as it happens, before the router reports its route.
  useEffect(() => connectRouterEntries(router, bridge, entries), [router, bridge, entries]);
  const focusedSessionId = useAtom(manager.projectionAtom, selectFocusedSessionId);
  const destinationRoute = useAtom(manager.projectionAtom, state =>
    selectPaletteDestinationRoute(state, paletteIntent?.windowId)
  );
  const focusedSession = useSession(focusedSessionId ?? "");

  useEffect(() => {
    manager.start();
    return () => manager.stop();
  }, [manager]);

  useEffect(
    () =>
      subscribeWorkspaceSwitchBarrier(
        shell.coordinator,
        () => manager.getState().client?.workspaceId ?? null
      ),
    [manager, shell]
  );

  // A client view held behind this client's focuses is authoritative state once adopted, like
  // one adopted on arrival below.
  useEffect(
    () => manager.onHeldClientAdopted(() => shell.coordinator.reportAuthoritativeState()),
    [manager, shell]
  );

  useEffect(() => {
    if (activeWorkspaceId === null) {
      manager.unbind();
      return undefined;
    }
    manager.bind({ workspaceId: activeWorkspaceId, profileId, clientId: client.clientId });
    shell.coordinator.beginWorkspaceSwitch();
    return () => manager.unbind();
  }, [activeWorkspaceId, client.clientId, manager, profileId, shell]);

  // Bind first: workspace selection and registration can settle in the same
  // commit, and setClient intentionally rejects a client for an unbound runtime.
  useEffect(() => {
    manager.setClient(client.client);
    manager.setLoadError(client.error);
    shell.coordinator.reportAuthoritativeState();
  }, [activeWorkspaceId, client.client, client.error, manager, profileId, shell]);

  // A re-registration after a lost one (a daemon restart) refocuses the window this page had
  // focused, once the restored view is adopted (#1655). The answer is authoritative state: its
  // report replaces the URL to that window and adds no history entry. The saved focus is kept
  // until the focus applies or the window is gone: a focus the daemon refuses (sent against a
  // snapshot older than the restarted daemon's) is tried again on the next snapshot revision.
  const restoredFocus = client.restoredFocus;
  const settleRestore = useEffectEvent(() => client.restoreSettled());
  const snapshotRevision = query.data?.revision ?? null;
  const restoreInFlight = useRef(false);
  useEffect(() => {
    if (restoredFocus === null || snapshotRevision === null || restoreInFlight.current) return;
    const state = manager.getState();
    if (state.client === null) return;
    if (!state.windows[restoredFocus] || state.focusedId === restoredFocus) {
      settleRestore();
      return;
    }
    restoreInFlight.current = true;
    void manager
      .focusWindow(restoredFocus, { replaceable: true })
      .completion.catch(() => false)
      .then(applied => {
        restoreInFlight.current = false;
        if (applied) settleRestore();
      });
  }, [client.client, manager, restoredFocus, snapshotRevision]);

  // After the rebind above, so the sweep reads the target desk rather than the one it replaced.
  useProfileSwitchWindowSweep(shell, activeWorkspaceId);

  useEffect(() => {
    if (client.status === "registered" && query.data && configQuery.data) {
      shell.coordinator.completeHydration();
    }
  }, [client.status, configQuery.data, query.data, shell]);

  useWindowManagerStream({
    workspaceId: activeWorkspaceId,
    profileId,
    clientId: client.clientId,
    registrationEpoch: client.registrationEpoch,
    currentClient: client.client,
    clientContext: resolveLivePaletteClientContext({
      breadthMode,
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
          queryKey: windowManagerKeys.config(configWorkspaceId, client.clientId, profileId),
          exact: true,
        });
      }
    },
    onClientCommand: (command, reply) => {
      return clientCommandChannel.execute(command.op, command.payload, reply);
    },
    // The view this page showed, read before the registration it belonged to is dropped.
    onClientInvalidated: () => client.reregister(clientRestore(manager.getState().client)),
    onError: error => manager.setLoadError(streamError(error)),
  });

  return {
    terminalClose,
    unsavedClose,
    shell,
    query,
    client: client.client,
    clientCommandChannel,
    profileId,
  };
}
