import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarShortcut,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "@compozy/ui";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";
import { usePaletteCommand, usePaletteRegistry } from "../../hooks/use-palette-registry";
import {
  MOVE_TO_DESKTOP_SLOT_COUNT,
  moveToDesktopSlotCommandId,
  useWindowMoveTargets,
  type WindowMoveTarget,
} from "../../hooks/use-window-move-targets";
import { moveFocusedWindowToDesktopId, SWAP_DIRECTIONS } from "../../lib/cmd-palette-window-ops";
import { activeTabCandidates, directionalSwapTarget } from "../../lib/window-manager-navigation";
import { MenubarCommandGroups } from "./menubar-command-groups";
import { MenubarCommandItem } from "./menubar-command-item";

export interface WindowMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

const MOVE_SLOT_COMMANDS = Array.from({ length: MOVE_TO_DESKTOP_SLOT_COUNT }, (_, index) =>
  moveToDesktopSlotCommandId(index + 1)
);
/**
 * Destinations past the ninth slot have no command of their own; they share
 * the move family's registry availability (a focused window), read from its
 * first slot, and move by desktop id through the same move boundary.
 */
const MOVE_FAMILY_COMMAND = MOVE_SLOT_COMMANDS[0]!;
/** Window › Arrange: the presets in `COPY.md` order, then Balance sizes. */
const ARRANGE_COMMANDS = [
  "layout.arrange.main-stack",
  "layout.arrange.columns",
  "layout.arrange.grid",
] as const;
const BALANCE_COMMAND = "layout.balance";
type SwapDirection = (typeof SWAP_DIRECTIONS)[number];
/** Window › Swap: one destination per direction, labelled by the side it trades with. */
const SWAP_LABELS: Record<SwapDirection, string> = {
  left: "Left",
  right: "Right",
  up: "Up",
  down: "Down",
};
/** The swap family shares one availability (a window in a split), read from its first id. */
const SWAP_FAMILY_COMMAND = "window.swap.left";
function swapCommandId(direction: SwapDirection): string {
  return `window.swap.${direction}`;
}
/** Why "Move to desktop" has nothing to offer on a single desktop. */
const MOVE_NEEDS_DESKTOP_REASON = "needs another desktop";
function menuGroup(id: string, content: React.ReactNode) {
  return { id, content };
}

/**
 * One "Move to desktop" destination. The desktop's name is the destination
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
 * One "Swap" destination. The side is the destination itself, not a second copy
 * of the command title; availability comes from the registry projection and the
 * item keeps its chord when one is bound.
 */
function SwapDirectionItem({
  direction,
  onRun,
}: {
  direction: SwapDirection;
  onRun: (commandId: string) => void;
}) {
  const commandId = swapCommandId(direction);
  const command = usePaletteCommand(commandId);
  if (command === null) return null;
  return (
    <MenubarItem
      data-testid={`os-menubar-command-${commandId}`}
      disabled={!command.available}
      onClick={() => onRun(commandId)}
    >
      <span className="min-w-0 flex-1 truncate">{SWAP_LABELS[direction]}</span>
      {command.chords.length > 0 ? <MenubarShortcut>{command.chords[0]}</MenubarShortcut> : null}
    </MenubarItem>
  );
}

/**
 * Window › Swap: the directions in which the focused window has an on-screen
 * neighbour in its tab. Outside a split the entry disables in place with the
 * registry's reason, as "Move to new tab" does; a side with no neighbour is absent.
 * Sides are read from the window the command will start from (a focus still
 * waiting for its answer included), so a listed side is one that swaps. The menu
 * content mounts on open, and every view change re-reads it.
 */
function SwapMenu({ onRun }: { onRun: (commandId: string) => void }) {
  const { manager } = useOsShell();
  const family = usePaletteCommand(SWAP_FAMILY_COMMAND);
  const directions = useDesktop(state => {
    const from = manager.directionalFocusId();
    const candidates = activeTabCandidates(state);
    return SWAP_DIRECTIONS.filter(
      direction => directionalSwapTarget(candidates, from, direction) !== null
    );
  }, sameDirections);
  if (family === null) return null;
  if (!family.available || directions.length === 0) {
    return (
      <MenubarItem data-testid="os-menu-swap" disabled>
        <span className="min-w-0 flex-1">
          <span className="block">Swap</span>
          {!family.available && family.reason ? (
            <span className="block text-eyebrow text-muted-foreground">{family.reason}</span>
          ) : null}
        </span>
      </MenubarItem>
    );
  }
  return (
    <MenubarSub>
      <MenubarSubTrigger data-testid="os-menu-swap">Swap</MenubarSubTrigger>
      <MenubarSubContent>
        {directions.map(direction => (
          <SwapDirectionItem key={direction} direction={direction} onRun={onRun} />
        ))}
      </MenubarSubContent>
    </MenubarSub>
  );
}

function sameDirections(
  left: readonly SwapDirection[] | undefined,
  right: readonly SwapDirection[]
): boolean {
  return (
    left !== undefined &&
    left.length === right.length &&
    left.every((direction, index) => direction === right[index])
  );
}

/**
 * Window menu: Move to new tab and Swap ▸ (a split member), Arrange ▸ with Balance, Move to
 * desktop (the focused window's whole tab), All desktops, Close. Grouping and order are curated
 * here (BR-17); directional focus and layout undo/redo stay palette and keyboard commands. Every
 * item's chord, availability and reason are projections of the registry, so an item shows the same
 * truth as its palette row and its chord.
 */
export function WindowMenu({ open, onOpenChange, onRun }: WindowMenuProps) {
  const registry = usePaletteRegistry();
  const moveTargets = useWindowMoveTargets();
  const { manager } = useOsShell();
  const moveTo = (desktopId: string) => moveFocusedWindowToDesktopId(manager, desktopId);
  const has = (commandId: string) => registry.byId.has(commandId);
  const reachableTargets = moveTargets.filter(target =>
    has(target.slotCommandId ?? MOVE_FAMILY_COMMAND)
  );
  const canMoveToDesktop = MOVE_SLOT_COMMANDS.some(has);
  const arrangeCommands = [...ARRANGE_COMMANDS, BALANCE_COMMAND].filter(has);
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Window</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-window">
        <MenubarCommandGroups
          groups={[
            menuGroup(
              "split",
              has("window.unsplit") || has(SWAP_FAMILY_COMMAND) ? (
                <>
                  {has("window.unsplit") ? (
                    <MenubarCommandItem commandId="window.unsplit" onRun={onRun} />
                  ) : null}
                  <SwapMenu onRun={onRun} />
                </>
              ) : null
            ),
            menuGroup(
              "arrange",
              arrangeCommands.length > 0 ? (
                <MenubarSub>
                  <MenubarSubTrigger data-testid="os-menu-arrange">Arrange</MenubarSubTrigger>
                  <MenubarSubContent>
                    {arrangeCommands.map(commandId => (
                      <MenubarCommandItem key={commandId} commandId={commandId} onRun={onRun} />
                    ))}
                  </MenubarSubContent>
                </MenubarSub>
              ) : null
            ),
            menuGroup(
              "move",
              reachableTargets.length > 0 ? (
                <MenubarSub>
                  <MenubarSubTrigger data-testid="os-menu-move-to-desktop">
                    Move to desktop
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
                    <span className="block">Move to desktop</span>
                    <span className="block text-eyebrow text-muted-foreground">
                      {MOVE_NEEDS_DESKTOP_REASON}
                    </span>
                  </span>
                </MenubarItem>
              ) : null
            ),
            menuGroup(
              "desktops",
              has("desktop.overview") ? (
                <MenubarCommandItem commandId="desktop.overview" onRun={onRun} />
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
