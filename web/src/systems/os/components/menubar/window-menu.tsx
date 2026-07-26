import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarShortcut,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "@agh/ui";

import type { OsWindowCommandsModel } from "../../hooks/use-os-window-commands";
import {
  WINDOW_ARRANGE_COMMANDS,
  WINDOW_MANAGER_ACTIONS,
  WINDOW_PLACEMENT_COMMANDS,
  type WindowManagerActionId,
} from "../../lib/window-manager-command-registry";
import type { FocusDirection } from "../../lib/window-manager-types";

export interface WindowMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  commands: OsWindowCommandsModel;
  onOpenDesktops: () => void;
}

const FOCUS_ACTIONS = WINDOW_MANAGER_ACTIONS.flatMap(action =>
  action.id.startsWith("window.focus.")
    ? [
        {
          id: action.id,
          label: action.label,
          direction: action.id.slice("window.focus.".length) as FocusDirection,
        },
      ]
    : []
);

/**
 * Window menu. Every item is present at all times and disabled for exactly one
 * runtime reason — no focused window, no live client, a non-floating
 * presentation, a missing arrange peer, or a single desktop. Shortcut glyphs
 * come from the resolved registry, so an unbound action shows no chord rather
 * than a fake one.
 */
export function WindowMenu({ open, onOpenChange, commands, onOpenDesktops }: WindowMenuProps) {
  const focused = commands.focusedWindowActions;
  const chord = (id: WindowManagerActionId) => commands.shortcutLabels[id];
  const canPlace = commands.placementCommands.length > 0;
  const canArrange = commands.arrangeCommands.length > 0;

  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Window</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-window">
        <MenubarItem
          data-testid="os-menu-minimize-window"
          disabled={focused === null}
          onClick={() => focused?.minimize()}
        >
          Minimize
          <Chord label={chord("window.minimize")} />
        </MenubarItem>
        <MenubarItem
          data-testid="os-menu-zoom-window"
          disabled={focused?.zoom == null}
          onClick={() => focused?.zoom?.()}
        >
          Zoom
          <Chord label={chord("window.zoom")} />
        </MenubarItem>
        <MenubarItem
          data-testid="os-menu-toggle-floating"
          disabled={!commands.canToggleFloating}
          onClick={commands.toggleFloating}
        >
          Toggle floating
          <Chord label={chord("window.toggle_floating")} />
        </MenubarItem>
        <MenubarSeparator />
        <MenubarSub>
          <MenubarSubTrigger data-testid="os-menu-move-resize" disabled={!canPlace}>
            Move &amp; resize
          </MenubarSubTrigger>
          <MenubarSubContent>
            {WINDOW_PLACEMENT_COMMANDS.map(command => (
              <MenubarItem
                key={command.id}
                data-testid={`os-menu-tile-${command.placement}`}
                disabled={!canPlace}
                onClick={() => commands.dispatchPlacement(command)}
              >
                {command.label}
                <Chord label={chord(command.id)} />
              </MenubarItem>
            ))}
          </MenubarSubContent>
        </MenubarSub>
        <MenubarSub>
          <MenubarSubTrigger
            data-testid="os-menu-arrange"
            disabled={!canArrange && !commands.canBalanceLayout}
          >
            Arrange
          </MenubarSubTrigger>
          <MenubarSubContent>
            {WINDOW_ARRANGE_COMMANDS.map(command => (
              <MenubarItem
                key={command.id}
                data-testid={`os-menu-arrange-${command.preset}`}
                disabled={!canArrange}
                onClick={() => commands.dispatchArrange(command)}
              >
                {command.label}
                <Chord label={chord(command.id)} />
              </MenubarItem>
            ))}
            <MenubarSeparator />
            <MenubarItem
              data-testid="os-menu-balance-layout"
              disabled={!commands.canBalanceLayout}
              onClick={commands.balanceLayout}
            >
              Balance layout
              <Chord label={chord("layout.balance")} />
            </MenubarItem>
          </MenubarSubContent>
        </MenubarSub>
        <MenubarSub>
          <MenubarSubTrigger
            data-testid="os-menu-focus-window"
            disabled={!commands.canFocusDirection}
          >
            Focus
          </MenubarSubTrigger>
          <MenubarSubContent>
            {FOCUS_ACTIONS.map(action => (
              <MenubarItem
                key={action.id}
                data-testid={`os-menu-focus-${action.direction}`}
                disabled={!commands.canFocusDirection}
                onClick={() => commands.focusDirection(action.direction)}
              >
                {action.label}
                <Chord label={chord(action.id)} />
              </MenubarItem>
            ))}
          </MenubarSubContent>
        </MenubarSub>
        <MenubarSeparator />
        <MenubarItem
          data-testid="os-menu-undo-layout"
          disabled={!commands.canEditLayoutHistory}
          onClick={commands.undoLayout}
        >
          Undo layout
          <Chord label={chord("layout.undo")} />
        </MenubarItem>
        <MenubarItem
          data-testid="os-menu-redo-layout"
          disabled={!commands.canEditLayoutHistory}
          onClick={commands.redoLayout}
        >
          Redo layout
          <Chord label={chord("layout.redo")} />
        </MenubarItem>
        <MenubarSeparator />
        <MenubarItem
          data-testid="os-menu-previous-desktop"
          disabled={!commands.canSwitchDesktop}
          onClick={() => commands.switchDesktop("previous")}
        >
          Previous desktop
          <Chord label={chord("desktop.switch.previous")} />
        </MenubarItem>
        <MenubarItem
          data-testid="os-menu-next-desktop"
          disabled={!commands.canSwitchDesktop}
          onClick={() => commands.switchDesktop("next")}
        >
          Next desktop
          <Chord label={chord("desktop.switch.next")} />
        </MenubarItem>
        <MenubarItem data-testid="os-menu-desktops-overview" onClick={onOpenDesktops}>
          Desktops overview
          <Chord label={chord("desktop.overview")} />
        </MenubarItem>
        <MenubarSeparator />
        <MenubarItem
          data-testid="os-menu-close-window"
          disabled={focused === null}
          onClick={() => focused?.close()}
        >
          Close window
          <Chord label={chord("window.close")} />
        </MenubarItem>
      </MenubarContent>
    </MenubarMenu>
  );
}

function Chord({ label }: { label: string | undefined }) {
  if (!label) return null;
  return <MenubarShortcut>{label}</MenubarShortcut>;
}
