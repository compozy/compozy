import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "@compozy/ui";
import type { ReactNode } from "react";

import type { OsTabBarModel, OsTabModel } from "../hooks/use-os-tab-bar";
import { useWindowMoveTargets } from "../hooks/use-window-move-targets";
import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";

/** Counts the unpinned tabs each bulk close would reach from this tab. */
function closeReach(tabs: readonly OsTabModel[], index: number) {
  const unpinned = (from: number, to: number) =>
    tabs.slice(from, to).filter(model => !model.tab.pinned).length;
  const self = tabs[index]?.tab.pinned ? 0 : 1;
  return {
    others: unpinned(0, tabs.length) - self,
    right: unpinned(index + 1, tabs.length),
    left: unpinned(0, index),
    all: unpinned(0, tabs.length),
  };
}

/**
 * The tab's context menu. Bulk closes skip pinned tabs; a pinned single-window tab has no Close
 * item — Unpin comes first. S5 adds the split items (Split with, Move left/right, unsplit).
 */
export function OsTabMenu({
  bar,
  model,
  children,
}: {
  bar: OsTabBarModel;
  model: OsTabModel;
  children: ReactNode;
}) {
  const moveTargets = useWindowMoveTargets();
  const tabId = model.tab.id;
  const pinned = model.tab.pinned;
  const index = bar.tabs.findIndex(entry => entry.tab.id === tabId);
  const reach = closeReach(bar.tabs, index);
  const closable = !pinned || model.members.length > 1;

  return (
    <ContextMenu>
      <ContextMenuTrigger
        data-testid={`os-tab-menu-${tabId}`}
        render={
          <div role="presentation" className="flex shrink-0" onKeyDown={openContextMenuFromKeyboard}>
            {children}
          </div>
        }
      />
      <ContextMenuContent>
        <ContextMenuItem onClick={() => bar.pin(tabId, !pinned)}>
          {pinned ? "Unpin tab" : "Pin tab"}
        </ContextMenuItem>
        {moveTargets.length > 0 ? (
          <ContextMenuSub>
            <ContextMenuSubTrigger>Move to desktop</ContextMenuSubTrigger>
            <ContextMenuSubContent>
              {moveTargets.map(target => (
                <ContextMenuItem
                  key={target.desktopId}
                  onClick={() => bar.moveToDesktop(tabId, target.desktopId)}
                >
                  {target.name}
                </ContextMenuItem>
              ))}
            </ContextMenuSubContent>
          </ContextMenuSub>
        ) : null}
        <ContextMenuSeparator />
        {closable ? (
          <ContextMenuItem onClick={() => bar.close(tabId)}>
            Close tab
            {bar.shortcutLabels.close ? (
              <ContextMenuShortcut>{bar.shortcutLabels.close}</ContextMenuShortcut>
            ) : null}
          </ContextMenuItem>
        ) : null}
        <ContextMenuItem
          disabled={reach.others === 0}
          onClick={() => bar.closeScoped(tabId, "others")}
        >
          Close other tabs
        </ContextMenuItem>
        <ContextMenuItem disabled={reach.right === 0} onClick={() => bar.closeScoped(tabId, "right")}>
          Close tabs to the right
        </ContextMenuItem>
        <ContextMenuItem disabled={reach.left === 0} onClick={() => bar.closeScoped(tabId, "left")}>
          Close tabs to the left
        </ContextMenuItem>
        <ContextMenuItem disabled={reach.all === 0} onClick={() => bar.closeScoped(tabId, "all")}>
          Close all tabs
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem onClick={bar.reopen}>
          Reopen closed tab
          {bar.shortcutLabels.reopen ? (
            <ContextMenuShortcut>{bar.shortcutLabels.reopen}</ContextMenuShortcut>
          ) : null}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}
