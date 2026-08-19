import {
  MenubarContent,
  MenubarMenu,
  MenubarSeparator,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "@compozy/ui";

import { usePaletteRegistry } from "../../hooks/use-palette-registry";
import { MenubarCommandItem } from "./menubar-command-item";

export interface WindowMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

const TILE_COMMANDS = [
  "window.tile.left",
  "window.tile.right",
  "window.tile.top",
  "window.tile.bottom",
  "window.tile.top-left",
  "window.tile.top-right",
  "window.tile.bottom-left",
  "window.tile.bottom-right",
];
const ARRANGE_COMMANDS = ["layout.arrange.two-up", "layout.arrange.grid"];
const FOCUS_COMMANDS = [
  "window.focus.left",
  "window.focus.right",
  "window.focus.up",
  "window.focus.down",
];

/**
 * Window menu. Grouping and order are curated here (BR-17); every item's label,
 * chord, availability and reason are projections of the registry, so an item
 * shows the same truth as its palette row and its chord. An unbound command
 * shows no chord rather than a fake one.
 */
export function WindowMenu({ open, onOpenChange, onRun }: WindowMenuProps) {
  const registry = usePaletteRegistry();
  const has = (commandId: string) => registry.byId.has(commandId);
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Window</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-window">
        <MenubarCommandItem commandId="window.minimize" onRun={onRun} />
        <MenubarCommandItem commandId="window.zoom" onRun={onRun} />
        <MenubarCommandItem commandId="window.toggle_floating" onRun={onRun} />
        <MenubarSeparator />
        {TILE_COMMANDS.some(has) ? (
          <MenubarSub>
            <MenubarSubTrigger data-testid="os-menu-move-resize">
              Move &amp; resize
            </MenubarSubTrigger>
            <MenubarSubContent>
              {TILE_COMMANDS.map(commandId => (
                <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
              ))}
            </MenubarSubContent>
          </MenubarSub>
        ) : null}
        {ARRANGE_COMMANDS.some(has) || has("layout.balance") ? (
          <MenubarSub>
            <MenubarSubTrigger data-testid="os-menu-arrange">Arrange</MenubarSubTrigger>
            <MenubarSubContent>
              {ARRANGE_COMMANDS.map(commandId => (
                <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
              ))}
              <MenubarSeparator />
              <MenubarCommandItem commandId="layout.balance" onRun={onRun} />
            </MenubarSubContent>
          </MenubarSub>
        ) : null}
        {FOCUS_COMMANDS.some(has) ? (
          <MenubarSub>
            <MenubarSubTrigger data-testid="os-menu-focus-window">Focus</MenubarSubTrigger>
            <MenubarSubContent>
              {FOCUS_COMMANDS.map(commandId => (
                <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
              ))}
            </MenubarSubContent>
          </MenubarSub>
        ) : null}
        <MenubarSeparator />
        <MenubarCommandItem commandId="window.merge_all" onRun={onRun} />
        <MenubarCommandItem commandId="window.tab.detach" onRun={onRun} />
        <MenubarSeparator />
        <MenubarCommandItem commandId="layout.undo" onRun={onRun} />
        <MenubarCommandItem commandId="layout.redo" onRun={onRun} />
        <MenubarSeparator />
        <MenubarCommandItem commandId="desktop.switch.previous" onRun={onRun} />
        <MenubarCommandItem commandId="desktop.switch.next" onRun={onRun} />
        <MenubarCommandItem commandId="desktop.overview" onRun={onRun} />
        <MenubarSeparator />
        <MenubarCommandItem commandId="window.close" onRun={onRun} />
      </MenubarContent>
    </MenubarMenu>
  );
}
