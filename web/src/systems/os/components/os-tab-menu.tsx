import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
  MenuTreeItems,
  tidyMenuTree,
  type MenuTreeNode,
} from "@compozy/ui";
import { use, useRef, useSyncExternalStore, type ReactNode } from "react";

import type { SessionPayload } from "@/systems/session";

import { OsTabBarContext } from "../contexts/os-tab-bar-context";
import type { OsTabBarModel, OsTabModel } from "../hooks/use-os-tab-bar";
import { osWindowTitle } from "../hooks/use-os-window-title";
import { useWindowMoveTargets, type WindowMoveTarget } from "../hooks/use-window-move-targets";
import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";
import type { OsArrangePreset, OsWindow } from "../lib/os-types";
import { adjacentTabIndex } from "../lib/window-manager-command-builders";
import { subscribeWindowSlotRegistry, windowSlotSnapshot } from "../lib/window-slot-registry";

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

/** Close ▸: the bulk closes, each skipping pinned tabs. */
const CLOSE_SCOPES: readonly {
  scope: "others" | "right" | "left" | "all";
  label: string;
}[] = [
  { scope: "others", label: "Other tabs" },
  { scope: "right", label: "Tabs to the right" },
  { scope: "left", label: "Tabs to the left" },
  { scope: "all", label: "All tabs" },
];

type SessionOf = (win: OsWindow) => SessionPayload | undefined;

/**
 * Where focus goes when a tab's menu closes: the tab it was opened from. The menu's trigger is the
 * plate wrapper, which takes no focus, and the menu's default return lands on the trigger's first
 * tabbable descendant; a tab off the bar's one roving stop has none, so focus fell out of the bar.
 * The return runs when this menu's popup unmounts, after its exit, which a quick next action
 * outruns. Focus still in this menu (its popup, or one of its submenus while no other context
 * menu is open) goes back to the tab; focus that already moved on stays there: into a window the
 * closing click landed in, or into any other menu that opened meanwhile (another tab's menu, the
 * menubar's, a dropdown), which taking focus back would close.
 */
function tabReturnFocus(plate: HTMLElement | null, popup: HTMLElement | null): HTMLElement | false {
  const tab = plate?.querySelector<HTMLElement>('[role="tab"]');
  if (!tab) return false;
  const doc = tab.ownerDocument;
  const focused = doc.activeElement;
  if (focused === null || focused === doc.body) return tab;
  if (popup !== null && popup.contains(focused)) return tab;
  if (focused.closest('[data-slot="context-menu-sub-content"]') === null) return false;
  // A submenu portals outside its menu: it is this menu's only while no other context menu is open.
  const anotherMenuOpen = Array.from(
    doc.querySelectorAll<HTMLElement>('[data-slot="context-menu-content"]')
  ).some(menu => menu !== popup);
  return anotherMenuOpen ? false : tab;
}

/** The live title of each tab's lead window, re-read whenever a window publishes a new label. */
function useTabTitles(models: readonly OsTabModel[], sessionOf: SessionOf): readonly string[] {
  // One string snapshot keeps the read stable between publications; NUL never occurs in a title.
  const joined = useSyncExternalStore(
    subscribeWindowSlotRegistry,
    () =>
      models
        .map(model => {
          const lead = model.members[0]!;
          return osWindowTitle(lead, sessionOf(lead), windowSlotSnapshot(lead.id));
        })
        .join("\0")
  );
  return models.length === 0 ? [] : joined.split("\0");
}

interface TabMenuTreeInput {
  bar: OsTabBarModel;
  model: OsTabModel;
  moveTargets: readonly WindowMoveTarget[];
  /** Every other tab of the bar, with its title, for Split with ▸. */
  splitTargets: readonly { model: OsTabModel; title: string }[];
}

/** Drops the items a condition left out. */
function present(nodes: readonly (MenuTreeNode | null)[]): MenuTreeNode[] {
  return nodes.filter(node => node !== null);
}

function actionItem(id: string, label: string, onSelect: () => void, hint?: string): MenuTreeNode {
  return { kind: "item", id, label, hint: hint || undefined, onSelect };
}

/**
 * The tab menu tree, in `COPY.md` order: Pin, Move ▸, Split with ▸, Arrange ▸, then Close tab and
 * Close ▸, then Reopen closed tab. Conditions leave an item out; `tidyMenuTree` drops what that
 * empties (a submenu with no item, a group with no desktop, a leading or doubled separator).
 */
function tabMenuTree({ bar, model, moveTargets, splitTargets }: TabMenuTreeInput): MenuTreeNode[] {
  const tabId = model.tab.id;
  const pinned = model.tab.pinned;
  const index = bar.tabs.findIndex(entry => entry.tab.id === tabId);
  const reach = closeReach(bar.tabs, index);
  const barTabs = bar.tabs.map(entry => entry.tab);
  const windowCount = bar.tabs.reduce((sum, entry) => sum + entry.members.length, 0);

  const move: MenuTreeNode = {
    kind: "sub",
    id: "move",
    label: "Move",
    testId: `os-tab-move-menu-${tabId}`,
    children: present([
      adjacentTabIndex(barTabs, tabId, "left") === null
        ? null
        : actionItem("left", "Left", () => bar.moveTab(tabId, "left")),
      adjacentTabIndex(barTabs, tabId, "right") === null
        ? null
        : actionItem("right", "Right", () => bar.moveTab(tabId, "right")),
      { kind: "separator", id: "move-sep" },
      {
        kind: "group",
        id: "to-desktop",
        label: "To desktop",
        children: moveTargets.map(target =>
          actionItem(target.desktopId, target.name, () =>
            bar.moveToDesktop(tabId, target.desktopId)
          )
        ),
      },
    ]),
  };
  const splitWith: MenuTreeNode = {
    kind: "sub",
    id: "split-with",
    label: "Split with",
    testId: `os-tab-split-with-menu-${tabId}`,
    children: splitTargets.map(({ model: other, title }): MenuTreeNode => ({
      kind: "item",
      id: other.tab.id,
      label: title,
      hint: other.members.length > 1 ? String(other.members.length) : undefined,
      testId: `os-tab-split-with-${other.tab.id}`,
      onSelect: () => bar.splitWith(tabId, other.tab.id),
    })),
  };
  const arrange: MenuTreeNode | null =
    windowCount > 1
      ? {
          kind: "sub",
          id: "arrange",
          label: "Arrange",
          testId: `os-tab-arrange-menu-${tabId}`,
          children: present([
            ...ARRANGE_ITEMS.map(item =>
              actionItem(item.preset, item.label, () => bar.arrange(tabId, item.preset))
            ),
            // Balance sizes evens a split; a single-window tab has none.
            ...(model.members.length > 1
              ? [
                  { kind: "separator", id: "arrange-sep" } as const,
                  actionItem("balance", "Balance sizes", () => bar.balance(tabId)),
                ]
              : []),
          ]),
        }
      : null;
  const closeScoped: MenuTreeNode = {
    kind: "sub",
    id: "close-scoped",
    label: "Close",
    testId: `os-tab-close-menu-${tabId}`,
    // A bulk close that would close nothing is absent, not disabled (US-006.EC-4, S7).
    children: CLOSE_SCOPES.filter(item => reach[item.scope] > 0).map(item =>
      actionItem(item.scope, item.label, () => bar.closeScoped(tabId, item.scope))
    ),
  };

  return tidyMenuTree(
    present([
      actionItem("pin", pinned ? "Unpin tab" : "Pin tab", () => bar.pin(tabId, !pinned)),
      move,
      splitWith,
      arrange,
      { kind: "separator", id: "close-sep" },
      // A pinned single-window tab has no Close tab: Unpin comes first.
      !pinned || model.members.length > 1
        ? actionItem("close", "Close tab", () => bar.close(tabId), bar.shortcutLabels.close)
        : null,
      closeScoped,
      { kind: "separator", id: "reopen-sep" },
      bar.canReopen
        ? actionItem("reopen", "Reopen closed tab", bar.reopen, bar.shortcutLabels.reopen)
        : null,
    ])
  );
}

/**
 * The menu's items, against the live bar. Mounted only while the menu shows, so the closed menu of
 * every tab does not re-render with the bar's tab list.
 */
function OsTabMenuItems({ model, sessionOf }: { model: OsTabModel; sessionOf: SessionOf }) {
  const bar = use(OsTabBarContext);
  const moveTargets = useWindowMoveTargets();
  const others = bar?.tabs.filter(entry => entry.tab.id !== model.tab.id) ?? [];
  const titles = useTabTitles(others, sessionOf);
  if (bar === null) return null;
  const splitTargets = others.map((other, index) => ({ model: other, title: titles[index]! }));
  return (
    <MenuTreeItems host="context" nodes={tabMenuTree({ bar, model, moveTargets, splitTargets })} />
  );
}

/**
 * The tab's context menu (`tabMenuTree`): Pin, Move ▸ (Left, Right, To desktop), Split with ▸,
 * Arrange ▸, Close tab, Close ▸ (the bulk closes), Reopen closed tab. An item that cannot apply is
 * absent and a submenu left empty with it (a single tab has no Split with, the first tab of its
 * range no Left, a bulk close that reaches no tab, Reopen with nothing closed). Bulk closes skip
 * pinned tabs; a pinned single-window tab has no Close tab — Unpin comes first. The items read the
 * bar from `OsTabBarContext`.
 */
export function OsTabMenu({
  model,
  sessionOf,
  children,
}: {
  model: OsTabModel;
  sessionOf: SessionOf;
  children: ReactNode;
}) {
  const tabId = model.tab.id;
  const plateRef = useRef<HTMLDivElement>(null);
  // Kept past unmount: the return focus is resolved while this popup is leaving the document.
  const popupRef = useRef<HTMLElement | null>(null);

  return (
    <ContextMenu>
      <ContextMenuTrigger
        ref={plateRef}
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
      <ContextMenuContent
        ref={(popup: HTMLDivElement | null) => {
          if (popup) popupRef.current = popup;
        }}
        finalFocus={() => tabReturnFocus(plateRef.current, popupRef.current)}
      >
        <OsTabMenuItems model={model} sessionOf={sessionOf} />
      </ContextMenuContent>
    </ContextMenu>
  );
}
