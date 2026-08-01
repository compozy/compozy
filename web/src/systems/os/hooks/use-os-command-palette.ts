import { shallowEqual } from "@xstate/store";
import { useEffect, useSyncExternalStore } from "react";

import { getSessionDisplayTitle, useSessionCreateActions, useSessions } from "@/systems/session";
import { useActiveWorkspace, type WorkspacePayload } from "@/systems/workspace";

import { isWaitingSession } from "../lib/attention-model";
import { getOsApp } from "../lib/app-registry";
import { frameForWindow } from "../lib/group-projection";
import type {
  WindowArrangeCommand,
  WindowManagerActionId,
  WindowPlacementCommand,
} from "../lib/window-manager-command-registry";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import {
  pruneWindowSlotStores,
  subscribeWindowSlotRegistry,
  windowSlotRegistryVersion,
  windowSlotSnapshot,
} from "../lib/window-slot-registry";
import { windowManagerStore } from "../stores/window-manager-store";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useOsWindowCommands, type OsFocusedWindowActions } from "./use-os-window-commands";
import { useWindowPaletteIntent } from "./use-window-manager-store";

type PaletteSession = NonNullable<ReturnType<typeof useSessions>["data"]>[number];

function consumeDestination(): void {
  windowManagerStore.trigger.paletteIntentCleared();
}

export interface OsPaletteTabResult {
  windowId: string;
  app: OsAppId;
  label: string;
  desktopName: string;
  needsInput: boolean;
  minimized: boolean;
}

export interface OsCommandPaletteModel {
  paletteSessions: PaletteSession[];
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | null;
  /**
   * Every open tab across windows and desktops (US-021); empty hides the
   * group. Selecting one switches desktop, focuses, and activates in one
   * action.
   */
  tabResults: readonly OsPaletteTabResult[];
  goToTab(windowId: string): void;
  /** Destination scope: the palette is picking a surface for one new tab (US-005). */
  destinationWindowId: string | null;
  pickDestination(target: { app: OsAppId; instanceKey?: string; route?: OsWindowRoute }): void;
  /** Menu twins of drag grouping (US-015); null = disabled with reason. */
  mergeAllWindows: (() => void) | null;
  moveTabToNewWindow: (() => void) | null;
  newTab(): void;
  reopenClosedTab(): void;
  placementCommands: readonly WindowPlacementCommand[];
  /** Arrange presets; empty without a second visible window (truthful UI). */
  arrangeCommands: readonly WindowArrangeCommand[];
  shortcutLabels: Readonly<Partial<Record<WindowManagerActionId, string>>>;
  /**
   * Lifecycle actions for the focused window — the guaranteed keyboard path
   * where browsers reserve ⌘W/⌘M (US-003.AC-4/EC-3). `zoom` is null in
   * compact presentation (the control has no meaning in a stack).
   */
  focusedWindowActions: OsFocusedWindowActions | null;
  openApp(app: OsAppId): void;
  jumpToSession(sessionId: string, agentName: string): void;
  dispatchPlacement(command: WindowPlacementCommand): void;
  dispatchArrange(command: WindowArrangeCommand): void;
  toggleSessions(): void;
  newSession(): void;
  openDesktops(): void;
  openAppearance(): void;
  switchWorkspace(workspaceId: string): void;
  commandsAvailable: boolean;
}

/**
 * Palette view-model: sources (apps come from the registry constant), the
 * focused-window snap surface (ADR-009) lifted from `useOsWindowCommands` so
 * the menubar cannot drift from it, and the run-and-close dispatch pattern.
 * Keeps `OsCommandPalette` purely presentational.
 */
export function useOsCommandPalette(
  open: boolean,
  onOpenChange: (open: boolean) => void,
  options: { onOpenDesktops?: () => void; onToggleSessions?: () => void } = {}
): OsCommandPaletteModel {
  const { manager, coordinator } = useOsShell();
  const { openForAgent } = useSessionCreateActions();
  const { workspaces, activeWorkspaceId, setActiveWorkspaceId } = useActiveWorkspace();
  const sessions = useSessions(activeWorkspaceId, {
    enabled: open && activeWorkspaceId !== null,
  });
  const windowCommands = useOsWindowCommands();
  const { commandsAvailable } = windowCommands;
  const paletteIntent = useWindowPaletteIntent();
  const destinationWindowId =
    open && paletteIntent?.kind === "destination" ? paletteIntent.windowId : null;
  const desktopData = useDesktop(
    state => ({
      desktops: state.desktops,
      windows: state.windows,
    }),
    shallowEqual
  );
  useSyncExternalStore(
    subscribeWindowSlotRegistry,
    windowSlotRegistryVersion,
    windowSlotRegistryVersion
  );
  const liveWindowIds = Object.keys(desktopData.windows).join("\0");
  useEffect(() => {
    if (open) pruneWindowSlotStores(new Set(liveWindowIds.split("\0")));
  }, [open, liveWindowIds]);
  const tabResults = (() => {
    if (!open) return [] as OsPaletteTabResult[];
    const desktopNames = new Map(desktopData.desktops.map(desktop => [desktop.id, desktop.name]));
    const sessionsById = new Map((sessions.data ?? []).map(session => [session.id, session]));
    const results: OsPaletteTabResult[] = [];
    for (const win of Object.values(desktopData.windows)) {
      if (win.app === "new-tab" && win.id === destinationWindowId) continue;
      const session = win.instanceKey !== null ? sessionsById.get(win.instanceKey) : undefined;
      const slot = windowSlotSnapshot(win.id);
      const label =
        win.app === "session" && session
          ? getSessionDisplayTitle(session)
          : typeof slot?.crumb === "string"
            ? slot.crumb
            : getOsApp(win.app).title;
      results.push({
        windowId: win.id,
        app: win.app,
        label,
        desktopName: desktopNames.get(win.desktopId) ?? "",
        needsInput: session !== undefined && isWaitingSession(session),
        minimized: win.minimized,
      });
    }
    return results.sort((left, right) => left.label.localeCompare(right.label));
  })();
  const desktopWindowCount = useDesktop(state => {
    const activeDesktopId = state.activeDesktopId;
    if (activeDesktopId === null) return 0;
    return Object.values(state.windows).filter(win => win.desktopId === activeDesktopId).length;
  });
  const focusedStacked = useDesktop(state => {
    const focusedId = state.focusedId;
    if (focusedId === null) return null;
    const frame = frameForWindow(state.frames, focusedId);
    return frame !== null && frame.members.length > 1 ? focusedId : null;
  });

  const run = (action: () => void) => {
    onOpenChange(false);
    action();
  };

  const windowActions = windowCommands.focusedWindowActions;
  const zoom = windowActions?.zoom ?? null;
  const makeFloating = windowActions?.makeFloating ?? null;
  const focusedWindowActions: OsFocusedWindowActions | null =
    windowActions === null
      ? null
      : {
          close: () => run(windowActions.close),
          minimize: () => run(windowActions.minimize),
          zoom: zoom === null ? null : () => run(zoom),
          makeFloating: makeFloating === null ? null : () => run(makeFloating),
        };

  return {
    focusedWindowActions,
    commandsAvailable,
    paletteSessions: (sessions.data ?? []).filter(
      session => typeof session.agent_name === "string" && session.agent_name.length > 0
    ),
    workspaces,
    activeWorkspaceId,
    tabResults,
    goToTab: windowId =>
      run(() => {
        if (commandsAvailable) void coordinator.userActivateWindow(windowId);
      }),
    destinationWindowId,
    pickDestination: target =>
      run(() => {
        if (!commandsAvailable || destinationWindowId === null) return;
        consumeDestination();
        void coordinator
          .userOpen({ ...target, stackTargetWindowId: destinationWindowId })
          .then(openedId => {
            // The empty tab hands its place to the picked surface — the open
            // joined its frame, so closing it leaves the destination behind.
            if (openedId !== null) void manager.closeWindow(destinationWindowId);
          });
      }),
    mergeAllWindows:
      desktopWindowCount >= 2
        ? () =>
            run(() => {
              const state = manager.getState();
              const activeDesktopId = state.activeDesktopId;
              const anchor = state.focusedId;
              if (activeDesktopId === null || anchor === null) return;
              const joiners = Object.values(state.windows)
                .filter(win => win.desktopId === activeDesktopId && win.id !== anchor)
                .sort((left, right) => left.id.localeCompare(right.id))
                .map(win => win.id);
              if (joiners.length > 0) manager.groupWindows(anchor, joiners);
            })
        : null,
    moveTabToNewWindow:
      focusedStacked !== null
        ? () =>
            run(() => {
              const outcome = manager.toggleFloating(focusedStacked);
              if (outcome.accepted) {
                void outcome.completion.then(applied => {
                  if (applied) void coordinator.userActivateWindow(focusedStacked);
                });
              }
            })
        : null,
    newTab: () =>
      run(() => {
        if (!commandsAvailable) return;
        const focusedId = manager.getState().focusedId;
        void coordinator
          .userOpen({
            app: "new-tab",
            ...(focusedId ? { stackTargetWindowId: focusedId } : {}),
          })
          .then(windowId => {
            if (windowId !== null) {
              windowManagerStore.trigger.paletteIntentRequested({
                intent: { kind: "destination", windowId },
              });
            }
          });
      }),
    reopenClosedTab: () =>
      run(() => {
        if (commandsAvailable) void manager.reopenWindow().completion;
      }),
    shortcutLabels: windowCommands.shortcutLabels,
    placementCommands: windowCommands.placementCommands,
    arrangeCommands: windowCommands.arrangeCommands,
    openApp: app =>
      run(() => {
        if (commandsAvailable) void coordinator.userOpen({ app });
      }),
    jumpToSession: (sessionId, agentName) =>
      run(() => {
        if (!commandsAvailable) return;
        void coordinator.userOpen({
          app: "session",
          instanceKey: sessionId,
          route: {
            pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(sessionId)}`,
            search: {},
          },
        });
      }),
    dispatchPlacement: command => run(() => windowCommands.dispatchPlacement(command)),
    dispatchArrange: command => run(() => windowCommands.dispatchArrange(command)),
    toggleSessions: () => run(() => options.onToggleSessions?.()),
    newSession: () => run(() => openForAgent("")),
    openDesktops: () => run(() => options.onOpenDesktops?.()),
    openAppearance: () =>
      run(() => {
        if (!commandsAvailable) return;
        void coordinator.userOpen({
          app: "settings",
          route: { pathname: "/settings/appearance", search: {} },
        });
      }),
    switchWorkspace: workspaceId => run(() => setActiveWorkspaceId(workspaceId)),
  };
}
