import {
  Columns2,
  Combine,
  LayoutGrid,
  Maximize2,
  Minus,
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

import { CommandGroup, CommandItem, CommandShortcut } from "@compozy/ui";

import type { OsCommandPaletteModel } from "../hooks/use-os-command-palette";
import type { WindowPlacementId } from "../lib/window-manager-command-registry";

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

interface OsCommandPaletteWindowActionsProps {
  destination: boolean;
  model: OsCommandPaletteModel;
}

/** Lifecycle, layout, and tab actions for the current focused window. */
export function OsCommandPaletteWindowActions({
  destination,
  model,
}: OsCommandPaletteWindowActionsProps) {
  if (
    destination ||
    (model.focusedWindowActions === null && model.placementCommands.length === 0)
  ) {
    return null;
  }

  const windowActions = model.focusedWindowActions;

  return (
    <CommandGroup heading="Window">
      <CommandItem
        value="new tab"
        data-testid="os-palette-new-tab"
        onSelect={model.newTab}
        disabled={!model.commandsAvailable}
      >
        <Plus className="size-3.5 text-muted" />
        New tab
        {model.shortcutLabels["window.tab.new"] ? (
          <CommandShortcut>{model.shortcutLabels["window.tab.new"]}</CommandShortcut>
        ) : null}
      </CommandItem>
      <CommandItem
        value="reopen closed tab"
        data-testid="os-palette-reopen-tab"
        onSelect={model.reopenClosedTab}
        disabled={!model.commandsAvailable}
      >
        <RefreshCcw className="size-3.5 text-muted" />
        Reopen closed tab
        {model.shortcutLabels["window.tab.reopen"] ? (
          <CommandShortcut>{model.shortcutLabels["window.tab.reopen"]}</CommandShortcut>
        ) : null}
      </CommandItem>
      <CommandItem
        value="merge all windows"
        data-testid="os-palette-merge-all"
        onSelect={() => model.mergeAllWindows?.()}
        disabled={model.mergeAllWindows === null}
      >
        <Combine className="size-3.5 text-muted" />
        {model.mergeAllWindows === null
          ? "Merge all windows (needs two windows)"
          : "Merge all windows"}
      </CommandItem>
      <CommandItem
        value="move tab to new window"
        data-testid="os-palette-move-tab"
        onSelect={() => model.moveTabToNewWindow?.()}
        disabled={model.moveTabToNewWindow === null}
      >
        <PanelsTopLeft className="size-3.5 text-muted" />
        {model.moveTabToNewWindow === null
          ? "Move tab to new window (no tab focused)"
          : "Move tab to new window"}
      </CommandItem>
      {windowActions === null ? null : (
        <>
          <CommandItem
            value="close window"
            data-testid="os-palette-close-window"
            onSelect={windowActions.close}
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
            onSelect={windowActions.minimize}
          >
            <Minus className="size-3.5 text-muted" />
            Minimize window
            {model.shortcutLabels["window.minimize"] ? (
              <CommandShortcut>{model.shortcutLabels["window.minimize"]}</CommandShortcut>
            ) : null}
          </CommandItem>
          {windowActions.zoom === null ? null : (
            <CommandItem
              value="zoom window"
              data-testid="os-palette-zoom-window"
              onSelect={windowActions.zoom}
            >
              <Maximize2 className="size-3.5 text-muted" />
              Zoom window
              {model.shortcutLabels["window.zoom"] ? (
                <CommandShortcut>{model.shortcutLabels["window.zoom"]}</CommandShortcut>
              ) : null}
            </CommandItem>
          )}
          {windowActions.makeFloating === null ? null : (
            <CommandItem
              value="make window floating"
              data-testid="os-palette-make-floating"
              onSelect={windowActions.makeFloating}
            >
              <PanelsTopLeft className="size-3.5 text-muted" />
              Make window floating
              {model.shortcutLabels["window.toggle_floating"] ? (
                <CommandShortcut>{model.shortcutLabels["window.toggle_floating"]}</CommandShortcut>
              ) : null}
            </CommandItem>
          )}
        </>
      )}
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
  );
}
