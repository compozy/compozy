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

import type { SessionPayload } from "@/systems/session";

import type { OsTabBarModel, OsTabModel } from "../hooks/use-os-tab-bar";
import { useWindowMoveTargets } from "../hooks/use-window-move-targets";
import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";
import type { OsArrangePreset, OsWindow } from "../lib/os-types";
import { adjacentTabIndex } from "../lib/window-manager-command-builders";
import { useOsWindowTitle } from "../hooks/use-os-window-title";

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

/** Window › Arrange presets, in `COPY.md` order and wording. */
const ARRANGE_ITEMS: readonly { preset: OsArrangePreset; label: string }[] = [
  { preset: "main-stack", label: "Main and stack" },
  { preset: "columns", label: "Columns" },
  { preset: "grid", label: "Grid" },
];

type SessionOf = (win: OsWindow) => SessionPayload | undefined;

/** One "Split with ▸" destination, named like its tab. */
function SplitWithItem({
  model,
  sessionOf,
  onSelect,
}: {
  model: OsTabModel;
  sessionOf: SessionOf;
  onSelect: () => void;
}) {
  const lead = model.members[0]!;
  const title = useOsWindowTitle(lead, sessionOf(lead));
  return (
    <ContextMenuItem data-testid={`os-tab-split-with-${model.tab.id}`} onClick={onSelect}>
      <span className="min-w-0 flex-1 truncate">{title}</span>
      {model.members.length > 1 ? (
        <span className="shrink-0 text-meta text-subtle tabular-nums">{model.members.length}</span>
      ) : null}
    </ContextMenuItem>
  );
}

/**
 * The tab's context menu: Pin, Move left / Move right, Split with ▸, Arrange ▸, Move to desktop ▸,
 * then the close set. An item that cannot apply is absent (a single tab has no Split with, the
 * first tab of its range no Move left, a bulk close that reaches no tab, Reopen with nothing
 * closed). Bulk closes skip pinned tabs; a pinned single-window tab has no Close item — Unpin
 * comes first.
 */
export function OsTabMenu({
  bar,
  model,
  sessionOf,
  children,
}: {
  bar: OsTabBarModel;
  model: OsTabModel;
  sessionOf: SessionOf;
  children: ReactNode;
}) {
  const moveTargets = useWindowMoveTargets();
  const tabId = model.tab.id;
  const pinned = model.tab.pinned;
  const index = bar.tabs.findIndex(entry => entry.tab.id === tabId);
  const reach = closeReach(bar.tabs, index);
  const closable = !pinned || model.members.length > 1;
  const barTabs = bar.tabs.map(entry => entry.tab);
  const canMoveLeft = adjacentTabIndex(barTabs, tabId, "left") !== null;
  const canMoveRight = adjacentTabIndex(barTabs, tabId, "right") !== null;
  const others = bar.tabs.filter(entry => entry.tab.id !== tabId);
  const windowCount = bar.tabs.reduce((sum, entry) => sum + entry.members.length, 0);

  return (
    <ContextMenu>
      <ContextMenuTrigger
        data-testid={`os-tab-menu-${tabId}`}
        render={
          <div
            role="presentation"
            className="flex shrink-0"
            onKeyDown={openContextMenuFromKeyboard}
          >
            {children}
          </div>
        }
      />
      <ContextMenuContent>
        <ContextMenuItem onClick={() => bar.pin(tabId, !pinned)}>
          {pinned ? "Unpin tab" : "Pin tab"}
        </ContextMenuItem>
        {canMoveLeft ? (
          <ContextMenuItem onClick={() => bar.moveTab(tabId, "left")}>Move left</ContextMenuItem>
        ) : null}
        {canMoveRight ? (
          <ContextMenuItem onClick={() => bar.moveTab(tabId, "right")}>Move right</ContextMenuItem>
        ) : null}
        {others.length > 0 ? (
          <ContextMenuSub>
            <ContextMenuSubTrigger data-testid={`os-tab-split-with-menu-${tabId}`}>
              Split with
            </ContextMenuSubTrigger>
            <ContextMenuSubContent>
              {others.map(other => (
                <SplitWithItem
                  key={other.tab.id}
                  model={other}
                  sessionOf={sessionOf}
                  onSelect={() => bar.splitWith(tabId, other.tab.id)}
                />
              ))}
            </ContextMenuSubContent>
          </ContextMenuSub>
        ) : null}
        {windowCount > 1 ? (
          <ContextMenuSub>
            <ContextMenuSubTrigger data-testid={`os-tab-arrange-menu-${tabId}`}>
              Arrange
            </ContextMenuSubTrigger>
            <ContextMenuSubContent>
              {ARRANGE_ITEMS.map(item => (
                <ContextMenuItem key={item.preset} onClick={() => bar.arrange(tabId, item.preset)}>
                  {item.label}
                </ContextMenuItem>
              ))}
              {model.members.length > 1 ? (
                <>
                  <ContextMenuSeparator />
                  <ContextMenuItem onClick={() => bar.balance(tabId)}>
                    Balance sizes
                  </ContextMenuItem>
                </>
              ) : null}
            </ContextMenuSubContent>
          </ContextMenuSub>
        ) : null}
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
        {/* A bulk close that would close nothing is absent, not disabled (US-006.EC-4, S7). */}
        {reach.others > 0 ? (
          <ContextMenuItem onClick={() => bar.closeScoped(tabId, "others")}>
            Close other tabs
          </ContextMenuItem>
        ) : null}
        {reach.right > 0 ? (
          <ContextMenuItem onClick={() => bar.closeScoped(tabId, "right")}>
            Close tabs to the right
          </ContextMenuItem>
        ) : null}
        {reach.left > 0 ? (
          <ContextMenuItem onClick={() => bar.closeScoped(tabId, "left")}>
            Close tabs to the left
          </ContextMenuItem>
        ) : null}
        {reach.all > 0 ? (
          <ContextMenuItem onClick={() => bar.closeScoped(tabId, "all")}>
            Close all tabs
          </ContextMenuItem>
        ) : null}
        {bar.canReopen ? (
          <>
            <ContextMenuSeparator />
            <ContextMenuItem onClick={bar.reopen}>
              Reopen closed tab
              {bar.shortcutLabels.reopen ? (
                <ContextMenuShortcut>{bar.shortcutLabels.reopen}</ContextMenuShortcut>
              ) : null}
            </ContextMenuItem>
          </>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}
