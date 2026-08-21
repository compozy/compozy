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
import { MenubarCommandGroups } from "./menubar-command-groups";
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

function menuGroup(id: string, content: React.ReactNode) {
  return { id, content };
}

/**
 * Window menu. Grouping and order are curated here (BR-17); every item's label,
 * chord, availability and reason are projections of the registry, so an item
 * shows the same truth as its palette row and its chord. An unbound command
 * shows no chord rather than a fake one.
 */
export function WindowMenu({ open, onOpenChange, onRun }: WindowMenuProps) {
  const registry = usePaletteRegistry();
  const has = (commandId: string) => registry.byId.has(commandId);
  const arrangeCommands = ARRANGE_COMMANDS.filter(has);
  const showBalance = has("layout.balance");
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Window</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-window">
        <MenubarCommandGroups
          groups={[
            menuGroup(
              "state",
              ["window.minimize", "window.zoom", "window.toggle_floating"].some(has) ? (
                <>
                  <MenubarCommandItem commandId="window.minimize" onRun={onRun} />
                  <MenubarCommandItem commandId="window.zoom" onRun={onRun} />
                  <MenubarCommandItem commandId="window.toggle_floating" onRun={onRun} />
                </>
              ) : null
            ),
            menuGroup(
              "tile",
              TILE_COMMANDS.some(has) ? (
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
              ) : null
            ),
            menuGroup(
              "arrange",
              arrangeCommands.length > 0 || showBalance ? (
                <MenubarSub>
                  <MenubarSubTrigger data-testid="os-menu-arrange">Arrange</MenubarSubTrigger>
                  <MenubarSubContent>
                    {arrangeCommands.map(commandId => (
                      <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
                    ))}
                    {arrangeCommands.length > 0 && showBalance ? <MenubarSeparator /> : null}
                    {showBalance ? (
                      <MenubarCommandItem commandId="layout.balance" onRun={onRun} />
                    ) : null}
                  </MenubarSubContent>
                </MenubarSub>
              ) : null
            ),
            menuGroup(
              "focus",
              FOCUS_COMMANDS.some(has) ? (
                <MenubarSub>
                  <MenubarSubTrigger data-testid="os-menu-focus-window">Focus</MenubarSubTrigger>
                  <MenubarSubContent>
                    {FOCUS_COMMANDS.map(commandId => (
                      <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
                    ))}
                  </MenubarSubContent>
                </MenubarSub>
              ) : null
            ),
            menuGroup(
              "stack",
              ["window.merge_all", "window.tab.detach"].some(has) ? (
                <>
                  <MenubarCommandItem commandId="window.merge_all" onRun={onRun} />
                  <MenubarCommandItem commandId="window.tab.detach" onRun={onRun} />
                </>
              ) : null
            ),
            menuGroup(
              "history",
              ["layout.undo", "layout.redo"].some(has) ? (
                <>
                  <MenubarCommandItem commandId="layout.undo" onRun={onRun} />
                  <MenubarCommandItem commandId="layout.redo" onRun={onRun} />
                </>
              ) : null
            ),
            menuGroup(
              "desktop",
              ["desktop.switch.previous", "desktop.switch.next", "desktop.overview"].some(has) ? (
                <>
                  <MenubarCommandItem commandId="desktop.switch.previous" onRun={onRun} />
                  <MenubarCommandItem commandId="desktop.switch.next" onRun={onRun} />
                  <MenubarCommandItem commandId="desktop.overview" onRun={onRun} />
                </>
              ) : null
            ),
            menuGroup(
              "close",
              has("window.close") ? (
                <MenubarCommandItem commandId="window.close" onRun={onRun} />
              ) : null
            ),
          ]}
        />
      </MenubarContent>
    </MenubarMenu>
  );
}
