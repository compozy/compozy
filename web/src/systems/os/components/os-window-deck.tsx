import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
  type TopbarSlotStore,
} from "@compozy/ui";
import { shallowEqual } from "@xstate/store";
import { Plus } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

import { useDesktop } from "../hooks/use-desktop";
import { useOsWindowDeck } from "../hooks/use-os-window-deck";
import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";
import type { OsWindowFrameModel } from "../lib/group-projection";
import type { OsWindow } from "../lib/os-types";
import { OsTrafficLights, type OsTrafficLightAction } from "./os-traffic-lights";
import { OsWindowTab } from "./os-window-tab";

function InsertionCaret() {
  return (
    <span
      role="presentation"
      aria-hidden="true"
      data-slot="os-window-deck-insertion"
      className="pointer-events-none h-deck-tab w-0.5 shrink-0 self-end rounded-full bg-accent"
    />
  );
}

function DeckTabMenu({
  win,
  frame,
  deck,
  children,
}: {
  win: OsWindow;
  frame: OsWindowFrameModel;
  deck: ReturnType<typeof useOsWindowDeck>;
  children: React.ReactNode;
}) {
  const pinnedByMember = useDesktop(state => {
    const map: Record<string, boolean> = {};
    for (const member of frame.members) map[member] = state.windows[member]?.pinned ?? false;
    return map;
  }, shallowEqual);
  const index = frame.members.indexOf(win.id);
  const unpinnedOthers = frame.members.filter(
    member => member !== win.id && !pinnedByMember[member]
  ).length;
  const unpinnedRight = frame.members
    .slice(index + 1)
    .filter(member => !pinnedByMember[member]).length;

  return (
    <ContextMenu>
      <ContextMenuTrigger
        data-testid={`os-window-tab-menu-${win.id}`}
        render={
          <div
            role="presentation"
            className="flex min-w-0 grow"
            onKeyDown={openContextMenuFromKeyboard}
          >
            {children}
          </div>
        }
      />
      <ContextMenuContent>
        <ContextMenuItem disabled={win.pinned} onClick={() => deck.closeTab(win.id)}>
          {win.pinned ? "Close tab (unpin first)" : "Close tab"}
          <ContextMenuShortcut>⌘W</ContextMenuShortcut>
        </ContextMenuItem>
        <ContextMenuItem disabled={unpinnedOthers === 0} onClick={() => deck.closeOthers(win.id)}>
          Close other tabs
        </ContextMenuItem>
        <ContextMenuItem disabled={unpinnedRight === 0} onClick={() => deck.closeRight(win.id)}>
          Close tabs to the right
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem onClick={() => deck.pinTab(win.id, !win.pinned)}>
          {win.pinned ? "Unpin tab" : "Pin tab"}
        </ContextMenuItem>
        <ContextMenuItem onClick={() => deck.moveToNewWindow(win.id)}>
          Move tab to new window
        </ContextMenuItem>
        <ContextMenuItem disabled={deck.otherWindowCount === 0} onClick={deck.mergeAllWindows}>
          Merge all windows
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}

export interface OsWindowDeckProps {
  frame: OsWindowFrameModel;
  slotStores: ReadonlyMap<string, TopbarSlotStore>;
  onTrafficLight: (action: OsTrafficLightAction) => void;
  zoomMenu?: (button: React.ReactNode) => React.ReactNode;
  dragHandleClassName: string;
}

/**
 * The deck (reference §01): a 37px row on the rail holding the OS controls,
 * one tab per member, the `+` button, and the drag gutter. Rendered only at
 * ≥2 members (D1); the active tab fuses with the head surface below.
 */
export function OsWindowDeck({
  frame,
  slotStores,
  onTrafficLight,
  zoomMenu,
  dragHandleClassName,
}: OsWindowDeckProps) {
  const deck = useOsWindowDeck(frame);
  const windows = useDesktop(state => state.windows);
  const indicatorIndex = deck.dropIndex ?? deck.tabDrag?.insertIndex ?? null;

  return (
    <div
      data-slot="os-window-deck"
      data-testid={`os-window-deck-${frame.id}`}
      className={cn(
        "flex h-deck shrink-0 cursor-grab items-end gap-0.5 bg-rail px-2.5 shadow-[inset_0_-1px_0_var(--color-line)] select-none active:cursor-grabbing",
        dragHandleClassName
      )}
    >
      <OsTrafficLights
        className="mr-1.5 h-deck-tab"
        onSelect={onTrafficLight}
        wrapZoom={zoomMenu}
      />
      <div
        ref={deck.tabsRef}
        role="tablist"
        aria-label="Open tabs"
        className="no-scrollbar flex min-w-0 items-end gap-0.5 overflow-x-auto"
      >
        {frame.members.map((member, index) => {
          const win = windows[member];
          if (!win) return null;
          return (
            <React.Fragment key={member}>
              {indicatorIndex === index ? <InsertionCaret /> : null}
              <div
                role="presentation"
                ref={element => deck.registerTab(member, element)}
                className={cn(
                  "flex",
                  win.pinned ? "min-w-0 shrink-0" : "w-deck-tab-max min-w-deck-tab",
                  deck.tabDrag?.windowId === member && "opacity-60"
                )}
              >
                <DeckTabMenu win={win} frame={frame} deck={deck}>
                  <OsWindowTab
                    win={win}
                    active={member === frame.activeWindowId}
                    session={
                      win.app === "session"
                        ? deck.sessionById.get(win.instanceKey ?? "")
                        : undefined
                    }
                    slotStore={slotStores.get(member)}
                    onActivate={() => {
                      if (!deck.wasDragged()) deck.activateTab(member);
                    }}
                    onClose={() => deck.closeTab(member)}
                    onCloseOthers={() => deck.closeOthers(member)}
                    onTabPointerDown={event => deck.handleTabPointerDown(member, event)}
                  />
                </DeckTabMenu>
              </div>
            </React.Fragment>
          );
        })}
        {indicatorIndex === frame.members.length ? <InsertionCaret /> : null}
      </div>
      <button
        type="button"
        aria-label="New tab (⌘T)"
        data-slot="os-window-tab-add"
        className="mb-[calc((var(--height-deck-tab)-var(--size-deck-add))/2)] grid size-deck-add shrink-0 place-items-center rounded-menubar-control text-subtle transition-colors duration-base hover:bg-btn-default-fill hover:text-fg-strong focus-visible:shadow-focus-ring focus-visible:outline-none"
        onClick={deck.openNewTab}
      >
        <Plus aria-hidden="true" className="size-3" strokeWidth={1.5} />
      </button>
      <span aria-hidden="true" className="min-w-4 flex-1" />
    </div>
  );
}
