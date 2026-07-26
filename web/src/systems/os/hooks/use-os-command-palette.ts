import { useSessionCreate, useSessions } from "@/systems/session";
import { useActiveWorkspace, type WorkspacePayload } from "@/systems/workspace";

import type {
  WindowArrangeCommand,
  WindowManagerActionId,
  WindowPlacementCommand,
} from "../lib/window-manager-command-registry";
import type { OsAppId } from "../lib/os-types";
import { useOsShell } from "./use-os-shell";
import { useOsWindowCommands, type OsFocusedWindowActions } from "./use-os-window-commands";

type PaletteSession = NonNullable<ReturnType<typeof useSessions>["data"]>[number];

export interface OsCommandPaletteModel {
  paletteSessions: PaletteSession[];
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | null;
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
  const { coordinator } = useOsShell();
  const sessionCreate = useSessionCreate();
  const { workspaces, activeWorkspaceId, setActiveWorkspaceId } = useActiveWorkspace();
  const sessions = useSessions(activeWorkspaceId, {
    enabled: open && activeWorkspaceId !== null,
  });
  const windowCommands = useOsWindowCommands();
  const { commandsAvailable } = windowCommands;

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
    newSession: () => run(() => sessionCreate.openForAgent("")),
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
