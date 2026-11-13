import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "@compozy/ui";
import type { ReactElement } from "react";

import type { SessionPayload } from "@/systems/session";

import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";
import type { OsWindow } from "../lib/os-types";
import { useOsWindowTitle } from "../hooks/use-os-window-title";
import type { DesktopOverviewItem, DesktopOverviewTab } from "./desktops-overview";

type SessionOf = (win: OsWindow) => SessionPayload | undefined;

/** One tab of "Move a tab ▸", named like its tab, opening the destination desktops. */
function MoveTabItem({
  tab,
  destinations,
  sessionOf,
  onMove,
}: {
  tab: DesktopOverviewTab;
  destinations: readonly DesktopOverviewItem[];
  sessionOf: SessionOf;
  onMove: (destinationId: string) => void;
}) {
  const lead = tab.members[0]!;
  const title = useOsWindowTitle(lead, sessionOf(lead));
  return (
    <ContextMenuSub>
      <ContextMenuSubTrigger>
        <span className="min-w-0 flex-1 truncate">{title}</span>
        {tab.members.length > 1 ? (
          <span className="shrink-0 text-meta text-subtle tabular-nums">{tab.members.length}</span>
        ) : null}
      </ContextMenuSubTrigger>
      <ContextMenuSubContent>
        <ContextMenuGroup>
          <ContextMenuLabel>Move to</ContextMenuLabel>
          {destinations.map(destination => (
            <ContextMenuItem key={destination.id} onClick={() => onMove(destination.id)}>
              {destination.name}
            </ContextMenuItem>
          ))}
        </ContextMenuGroup>
      </ContextMenuSubContent>
    </ContextMenuSub>
  );
}

export interface DesktopOverviewCardMenuProps {
  desktop: DesktopOverviewItem;
  desktops: readonly DesktopOverviewItem[];
  index: number;
  disabled: boolean;
  sessionOf: SessionOf;
  onRename: () => void;
  onReorder: (order: number) => void;
  onMoveTab: (leadWindowId: string, destinationDesktopId: string) => void;
  /** The card; right-click or the platform menu keys on a focused control open the menu. */
  children: ReactElement;
}

/**
 * The card's context menu — the keyboard path for what the card does by pointer: Rename,
 * Move left, Move right (absent at either end), and Move a tab ▸ to another desktop.
 */
export function DesktopOverviewCardMenu({
  desktop,
  desktops,
  index,
  disabled,
  sessionOf,
  onRename,
  onReorder,
  onMoveTab,
  children,
}: DesktopOverviewCardMenuProps) {
  const destinations = desktops.filter(candidate => candidate.id !== desktop.id);
  return (
    <ContextMenu disabled={disabled}>
      <ContextMenuTrigger
        render={children}
        onKeyDown={openContextMenuFromKeyboard}
        data-testid={`desktop-card-menu-${desktop.id}`}
      />
      <ContextMenuContent>
        <ContextMenuItem onClick={onRename}>Rename</ContextMenuItem>
        {index > 0 ? (
          <ContextMenuItem onClick={() => onReorder(index - 1)}>Move left</ContextMenuItem>
        ) : null}
        {index < desktops.length - 1 ? (
          <ContextMenuItem onClick={() => onReorder(index + 1)}>Move right</ContextMenuItem>
        ) : null}
        {desktop.tabs.length > 0 && destinations.length > 0 ? (
          <ContextMenuSub>
            <ContextMenuSubTrigger>Move a tab</ContextMenuSubTrigger>
            <ContextMenuSubContent>
              {desktop.tabs.map(tab => (
                <MoveTabItem
                  destinations={destinations}
                  key={tab.id}
                  sessionOf={sessionOf}
                  tab={tab}
                  onMove={destinationId => onMoveTab(tab.leadWindowId, destinationId)}
                />
              ))}
            </ContextMenuSubContent>
          </ContextMenuSub>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}
