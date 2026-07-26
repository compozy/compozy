import {
  Columns2,
  LayoutGrid,
  Maximize2,
  Minus,
  Palette,
  PanelBottom,
  PanelLeft,
  PanelRight,
  PanelTop,
  PanelsTopLeft,
  Plus,
  RefreshCcw,
  SquareArrowDownLeft,
  SquareArrowDownRight,
  SquareArrowUpLeft,
  SquareArrowUpRight,
  X,
  type LucideIcon,
} from "lucide-react";

import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from "@agh/ui";

import { useOsCommandPalette } from "../hooks/use-os-command-palette";
import { OS_APPS } from "../lib/app-registry";
import type { WindowPlacementId } from "../lib/window-manager-command-registry";

export interface OsCommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Opens the daemon-backed Desktops overview (⇧⌘S parity). */
  onOpenDesktops?: () => void;
  /** Toggles the global Sessions catalog modal. */
  onToggleSessions?: () => void;
}

/** Apps the palette can open: every registry app except multi-instance sessions. */
const PALETTE_APPS = Object.values(OS_APPS).filter(app => app.id !== "session");

const PLACEMENT_COMMAND_ICONS: Record<WindowPlacementId, LucideIcon> = {
  left: PanelLeft,
  right: PanelRight,
  top: PanelTop,
  bottom: PanelBottom,
  "top-left": SquareArrowUpLeft,
  "top-right": SquareArrowUpRight,
  "bottom-left": SquareArrowDownLeft,
  "bottom-right": SquareArrowDownRight,
};

const ARRANGE_COMMAND_ICONS: Record<"two-up" | "grid", LucideIcon> = {
  "two-up": Columns2,
  grid: LayoutGrid,
};

/**
 * The global ⌘K palette (ADR-005): apps, live sessions, window snap actions
 * (ADR-009 — the guaranteed keyboard path), and shell actions. A desktop-level
 * overlay (closed set) — it renders above the win-layer and portals to the
 * document body, never into a window. Behavior lives in `useOsCommandPalette`.
 */
export function OsCommandPalette({
  open,
  onOpenChange,
  onOpenDesktops,
  onToggleSessions,
}: OsCommandPaletteProps) {
  const model = useOsCommandPalette(open, onOpenChange, {
    onOpenDesktops,
    onToggleSessions,
  });

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Command palette"
      description="Search apps, sessions, and actions"
      // Compact clamps to the phone canvas (os-v2.css `.palette-overlay{padding-top:9vh}`).
      className="top-[9vh] min-[960px]:top-[16vh] sm:max-w-(--width-modal-sm)"
    >
      <Command data-testid="os-command-palette" shouldFilter>
        <CommandInput autoFocus placeholder="Search apps, sessions, actions…" />
        <CommandList className="max-h-[46vh]">
          <CommandEmpty>No matches — try an app, a session title, or an action.</CommandEmpty>
          <CommandGroup heading="Apps">
            {PALETTE_APPS.map(app => (
              <CommandItem
                key={app.id}
                value={`open ${app.title}`}
                data-testid={`os-palette-app-${app.id}`}
                onSelect={() => model.openApp(app.id)}
                disabled={!model.commandsAvailable}
              >
                <app.icon className="size-3.5 text-muted" />
                Open {app.title}
              </CommandItem>
            ))}
          </CommandGroup>
          {model.paletteSessions.length > 0 ? (
            <CommandGroup heading="Sessions">
              {model.paletteSessions.map(session => (
                <CommandItem
                  key={session.id}
                  value={`session ${session.name ?? session.id} ${session.agent_name}`}
                  data-testid={`os-palette-session-${session.id}`}
                  onSelect={() => model.jumpToSession(session.id, session.agent_name ?? "")}
                  disabled={!model.commandsAvailable}
                >
                  <OS_APPS.session.icon className="size-3.5 text-muted" />
                  <span className="min-w-0 truncate">{session.name?.trim() || session.id}</span>
                  <span className="ml-auto text-micro text-subtle">{session.agent_name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          ) : null}
          {model.focusedWindowActions !== null || model.placementCommands.length > 0 ? (
            <CommandGroup heading="Window">
              {model.focusedWindowActions !== null ? (
                <>
                  <CommandItem
                    value="close window"
                    data-testid="os-palette-close-window"
                    onSelect={model.focusedWindowActions.close}
                  >
                    <X className="size-3.5 text-muted" />
                    Close window
                    {model.shortcutLabels["window.close"] ? (
                      <CommandShortcut>{model.shortcutLabels["window.close"]}</CommandShortcut>
                    ) : null}
                  </CommandItem>
                  <CommandItem
                    value="minimize window"
                    data-testid="os-palette-minimize-window"
                    onSelect={model.focusedWindowActions.minimize}
                  >
                    <Minus className="size-3.5 text-muted" />
                    Minimize window
                    {model.shortcutLabels["window.minimize"] ? (
                      <CommandShortcut>{model.shortcutLabels["window.minimize"]}</CommandShortcut>
                    ) : null}
                  </CommandItem>
                  {model.focusedWindowActions.zoom !== null ? (
                    <CommandItem
                      value="zoom window"
                      data-testid="os-palette-zoom-window"
                      onSelect={model.focusedWindowActions.zoom}
                    >
                      <Maximize2 className="size-3.5 text-muted" />
                      Zoom window
                      {model.shortcutLabels["window.zoom"] ? (
                        <CommandShortcut>{model.shortcutLabels["window.zoom"]}</CommandShortcut>
                      ) : null}
                    </CommandItem>
                  ) : null}
                  {model.focusedWindowActions.makeFloating !== null ? (
                    <CommandItem
                      value="make window floating"
                      data-testid="os-palette-make-floating"
                      onSelect={model.focusedWindowActions.makeFloating}
                    >
                      <PanelsTopLeft className="size-3.5 text-muted" />
                      Make window floating
                      {model.shortcutLabels["window.toggle_floating"] ? (
                        <CommandShortcut>
                          {model.shortcutLabels["window.toggle_floating"]}
                        </CommandShortcut>
                      ) : null}
                    </CommandItem>
                  ) : null}
                </>
              ) : null}
              {model.placementCommands.map(command => {
                const Icon = PLACEMENT_COMMAND_ICONS[command.placement];
                return (
                  <CommandItem
                    key={command.id}
                    value={command.label}
                    data-testid={`os-palette-tile-${command.placement}`}
                    onSelect={() => model.dispatchPlacement(command)}
                  >
                    <Icon className="size-3.5 text-muted" />
                    {command.label}
                    {model.shortcutLabels[command.id] ? (
                      <CommandShortcut>{model.shortcutLabels[command.id]}</CommandShortcut>
                    ) : null}
                  </CommandItem>
                );
              })}
              {model.arrangeCommands.map(command => {
                const Icon = ARRANGE_COMMAND_ICONS[command.preset];
                return (
                  <CommandItem
                    key={command.preset}
                    value={command.label}
                    data-testid={`os-palette-arrange-${command.preset}`}
                    onSelect={() => model.dispatchArrange(command)}
                  >
                    <Icon className="size-3.5 text-muted" />
                    {command.label}
                    {model.shortcutLabels[command.id] ? (
                      <CommandShortcut>{model.shortcutLabels[command.id]}</CommandShortcut>
                    ) : null}
                  </CommandItem>
                );
              })}
            </CommandGroup>
          ) : null}
          <CommandGroup heading="Actions">
            <CommandItem
              value="toggle sessions"
              data-testid="os-palette-toggle-sessions"
              onSelect={model.toggleSessions}
            >
              <OS_APPS.session.icon className="size-3.5 text-muted" />
              Toggle sessions
            </CommandItem>
            <CommandItem
              value="new session"
              data-testid="os-palette-new-session"
              onSelect={model.newSession}
            >
              <Plus className="size-3.5 text-muted" />
              New session
              <CommandShortcut>⌘N</CommandShortcut>
            </CommandItem>
            <CommandItem
              value="desktops overview"
              data-testid="os-palette-desktops-overview"
              onSelect={model.openDesktops}
            >
              <PanelsTopLeft className="size-3.5 text-muted" />
              Desktops overview
              {model.shortcutLabels["desktop.overview"] ? (
                <CommandShortcut>{model.shortcutLabels["desktop.overview"]}</CommandShortcut>
              ) : null}
            </CommandItem>
            <CommandItem
              value="appearance wallpaper motion"
              data-testid="os-palette-appearance"
              onSelect={model.openAppearance}
              disabled={!model.commandsAvailable}
            >
              <Palette className="size-3.5 text-muted" />
              Appearance
            </CommandItem>
            {model.workspaces.flatMap(workspace =>
              workspace.id === model.activeWorkspaceId
                ? []
                : [
                    <CommandItem
                      key={workspace.id}
                      value={`switch to ${workspace.name}`}
                      data-testid={`os-palette-workspace-${workspace.id}`}
                      onSelect={() => model.switchWorkspace(workspace.id)}
                    >
                      <RefreshCcw className="size-3.5 text-muted" />
                      Switch to {workspace.name}
                    </CommandItem>,
                  ]
            )}
          </CommandGroup>
        </CommandList>
      </Command>
    </CommandDialog>
  );
}
