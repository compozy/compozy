import { Palette, PanelsTopLeft, Plus, RefreshCcw, ToggleRight } from "lucide-react";

import { CommandGroup, CommandItem, CommandShortcut } from "@compozy/ui";

import type { OsCommandPaletteModel } from "../hooks/use-os-command-palette";
import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";

interface OsCommandPaletteShellActionsProps {
  destination: boolean;
  model: OsCommandPaletteModel;
}

/** Shell-wide actions that remain outside a scoped new-tab destination picker. */
export function OsCommandPaletteShellActions({
  destination,
  model,
}: OsCommandPaletteShellActionsProps) {
  if (destination) return null;

  return (
    <CommandGroup heading="Actions">
      <CommandItem
        value="toggle sessions"
        data-testid="os-palette-toggle-sessions"
        onSelect={model.toggleSessions}
      >
        <OS_APP_DESCRIPTORS.session.icon className="size-3.5 text-muted" />
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
        disabled={model.scopeToggleLocked || (model.globalScopeOn && !model.canDisableGlobal)}
        value="toggle global scope"
        data-testid="os-palette-toggle-global-scope"
        onSelect={model.toggleGlobalScope}
      >
        <ToggleRight className="size-3.5 text-muted" />
        {model.globalScopeOn
          ? GLOBAL_SCOPE_COPY.paletteToggleOff
          : GLOBAL_SCOPE_COPY.paletteToggleOn}
        <CommandShortcut>⇧⌘G</CommandShortcut>
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
                {model.globalScopeOn ? (
                  <span className="ml-auto text-micro text-faint">
                    {GLOBAL_SCOPE_COPY.paletteSwitchTurnsOff}
                  </span>
                ) : null}
              </CommandItem>,
            ]
      )}
    </CommandGroup>
  );
}
