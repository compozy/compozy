import {
  MenubarContent,
  MenubarGroup,
  MenubarItem,
  MenubarLabel,
  MenubarMenu,
  MenubarShortcut,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "@compozy/ui";

import { useOsShell } from "../../hooks/use-os-shell";
import { usePaletteCommand, usePaletteRegistry } from "../../hooks/use-palette-registry";
import {
  MOVE_TO_DESKTOP_SLOT_COUNT,
  moveToDesktopSlotCommandId,
  useWindowMoveTargets,
  type WindowMoveTarget,
} from "../../hooks/use-window-move-targets";
import { moveFocusedWindowToDesktopId } from "../../lib/cmd-palette-window-ops";
import { MenubarCommandGroups } from "./menubar-command-groups";
import { MenubarCommandItem } from "./menubar-command-item";

export interface WindowMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

const ARRANGE_COMMANDS = [
  "layout.arrange.main-stack",
  "layout.arrange.columns",
  "layout.arrange.grid",
  "layout.balance",
];
const MOVE_SLOT_COMMANDS = Array.from({ length: MOVE_TO_DESKTOP_SLOT_COUNT }, (_, index) =>
  moveToDesktopSlotCommandId(index + 1)
);
/**
 * Destinations past the ninth slot have no command of their own; they share
 * the move family's registry availability (a focused window), read from its
 * first slot, and move by desktop id through the same move boundary.
 */
const MOVE_FAMILY_COMMAND = MOVE_SLOT_COMMANDS[0]!;
/** Why "Move window to" has nothing to offer on a single desktop. */
const MOVE_NEEDS_DESKTOP_REASON = "needs another desktop";
const STATE_COMMANDS = ["window.zoom", "window.minimize", "window.toggle_floating"];
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

function menuGroup(id: string, content: React.ReactNode) {
  return { id, content };
}

/**
 * One "Move window to" destination. The desktop's name is the destination
 * itself, not a second copy of the command title; availability and reason come
 * from the registry projection, and a slot destination keeps its chord.
 */
function MoveToDesktopItem({
  target,
  onRun,
  onMove,
}: {
  target: WindowMoveTarget;
  onRun: (commandId: string) => void;
  onMove: (desktopId: string) => void;
}) {
  const slotCommandId = target.slotCommandId;
  const command = usePaletteCommand(slotCommandId ?? MOVE_FAMILY_COMMAND);
  if (command === null) return null;
  return (
    <MenubarItem
      data-testid={
        slotCommandId === null
          ? `os-menubar-move-to-desktop-${target.desktopId}`
          : `os-menubar-command-${slotCommandId}`
      }
      disabled={!command.available}
      onClick={() => (slotCommandId === null ? onMove(target.desktopId) : onRun(slotCommandId))}
    >
      <span className="min-w-0 flex-1 truncate">{target.name}</span>
      {slotCommandId !== null && command.chords.length > 0 ? (
        <MenubarShortcut>{command.chords[0]}</MenubarShortcut>
      ) : null}
    </MenubarItem>
  );
}

/**
 * Window menu, in the shell-rail order: Arrange presets, Move window to, the
 * window's own state (with tiling under Move & resize), All desktops, Close.
 * Grouping and order are curated here (BR-17); directional focus, tab
 * merge/detach and layout undo/redo stay palette and keyboard commands. Every
 * item's label, chord, availability and reason are projections of the
 * registry, so an item shows the same truth as its palette row and its chord.
 */
export function WindowMenu({ open, onOpenChange, onRun }: WindowMenuProps) {
  const registry = usePaletteRegistry();
  const moveTargets = useWindowMoveTargets();
  const { manager } = useOsShell();
  const moveTo = (desktopId: string) => moveFocusedWindowToDesktopId(manager, desktopId);
  const has = (commandId: string) => registry.byId.has(commandId);
  const arrangeCommands = ARRANGE_COMMANDS.filter(has);
  const reachableTargets = moveTargets.filter(target =>
    has(target.slotCommandId ?? MOVE_FAMILY_COMMAND)
  );
  const canMoveToDesktop = MOVE_SLOT_COMMANDS.some(has);
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Window</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-window">
        <MenubarCommandGroups
          groups={[
            menuGroup(
              "arrange",
              arrangeCommands.length > 0 ? (
                <MenubarGroup data-testid="os-menu-arrange">
                  <MenubarLabel>Arrange</MenubarLabel>
                  {arrangeCommands.map(commandId => (
                    <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
                  ))}
                </MenubarGroup>
              ) : null
            ),
            menuGroup(
              "move",
              reachableTargets.length > 0 ? (
                <MenubarSub>
                  <MenubarSubTrigger data-testid="os-menu-move-to-desktop">
                    Move window to
                  </MenubarSubTrigger>
                  <MenubarSubContent>
                    {reachableTargets.map(target => (
                      <MoveToDesktopItem
                        key={target.desktopId}
                        onMove={moveTo}
                        onRun={onRun}
                        target={target}
                      />
                    ))}
                  </MenubarSubContent>
                </MenubarSub>
              ) : canMoveToDesktop ? (
                // One desktop: the destination list is empty, so the entry
                // stays in place disabled with its reason instead of vanishing.
                <MenubarItem data-testid="os-menu-move-to-desktop" disabled>
                  <span className="min-w-0 flex-1">
                    <span className="block">Move window to</span>
                    <span className="block text-eyebrow text-muted">
                      {MOVE_NEEDS_DESKTOP_REASON}
                    </span>
                  </span>
                </MenubarItem>
              ) : null
            ),
            menuGroup(
              "state",
              [...STATE_COMMANDS, ...TILE_COMMANDS, "desktop.overview"].some(has) ? (
                <>
                  {STATE_COMMANDS.map(commandId => (
                    <MenubarCommandItem commandId={commandId} key={commandId} onRun={onRun} />
                  ))}
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
