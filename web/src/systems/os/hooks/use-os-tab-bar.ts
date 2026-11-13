import { shallowEqual } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useEffect, useState } from "react";

import { type SessionPayload, useSessions } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import type { OsArrangePreset, OsCloseScope, OsWindow } from "../lib/os-types";
import { leadWindowId, resolveActiveTabId, tabWindowIds } from "../lib/tab-model";
import { allocateTabWidths, type TabWidthAllocation } from "../lib/tab-width";
import { shortcutActionLabel } from "../lib/window-manager-shortcuts";
import type { LayoutTab } from "../lib/window-manager-types";
import { osTabDragStore } from "../stores/os-tab-drag-store";
import { windowManagerStore } from "../stores/window-manager-store";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

/** One window's icon tile: a split stacks up to three, the lead in front. */
export const TAB_TILE_WIDTH = 24;
export const TAB_TILE_PEEK = 4;
export const TAB_TILE_STACK_MAX = 3;
/** Space between adjacent tabs and before the new-tab button (`gap-1`). */
const TAB_GAP = 4;
/** The new-tab button that follows the last tab (`size-6.75`). */
const NEW_TAB_BUTTON_WIDTH = 27;

/** A pinned tab hugs its tiles: the stack plus the 3.5px inset on each side. */
export function pinnedTabWidth(memberCount: number): number {
  return TAB_TILE_WIDTH + TAB_TILE_PEEK * (Math.min(memberCount, TAB_TILE_STACK_MAX) - 1) + 7;
}

export interface OsTabModel {
  tab: LayoutTab;
  active: boolean;
  /** The member this client focused last, else the first: names the tab and takes focus. */
  leadId: string;
  /** Members in reading order, lead first. */
  members: readonly OsWindow[];
}

function sameTabModels(left: readonly OsTabModel[] | undefined, right: readonly OsTabModel[]) {
  if (left === undefined || left.length !== right.length) return false;
  return left.every((model, index) => {
    const other = right[index]!;
    return (
      model.tab === other.tab &&
      model.active === other.active &&
      model.leadId === other.leadId &&
      model.members.length === other.members.length &&
      model.members.every((member, memberIndex) => member === other.members[memberIndex])
    );
  });
}

/**
 * The bar order a reorder drop committed, held until the daemon answers so the tab never jumps
 * back for a round trip. Tabs the held order does not know keep their place after it.
 */
function withSettlingOrder(
  models: readonly OsTabModel[],
  order: readonly string[] | null
): readonly OsTabModel[] {
  if (order === null) return models;
  const rank = (model: OsTabModel) => {
    const index = order.indexOf(model.tab.id);
    return index < 0 ? order.length : index;
  };
  return [...models].sort((left, right) => rank(left) - rank(right));
}

/** The active desktop's tabs in bar order, each with its members and lead window. */
export function useOsTabModels(): readonly OsTabModel[] {
  const activeDesktopId = useDesktop(state => state.activeDesktopId);
  const settlingOrder = useSelector(osTabDragStore, snapshot => {
    const settling = snapshot.context.settling;
    return settling !== null && settling.desktopId === activeDesktopId ? settling.order : null;
  });
  const models = useDesktop(state => {
    const desktop = state.desktops.find(entry => entry.id === state.activeDesktopId);
    if (desktop === undefined) return [];
    const activeTabId = state.activeTabIds[desktop.id] ?? resolveActiveTabId(desktop, null);
    const focusOrder = state.client?.focusOrder ?? [];
    return desktop.tabs.flatMap(tab => {
      const leadId = leadWindowId(tab, focusOrder);
      if (leadId === null) return [];
      const ordered = [leadId, ...tabWindowIds(tab).filter(id => id !== leadId)];
      const members = ordered.flatMap(id => (state.windows[id] ? [state.windows[id]] : []));
      if (members.length === 0) return [];
      return [{ tab, active: tab.id === activeTabId, leadId, members }];
    });
  }, sameTabModels);
  return withSettlingOrder(models, settlingOrder);
}

/** Live sessions by id, read only while some tab holds a session window. */
export function useTabSessions(hasSession: boolean): ReadonlyMap<string, SessionPayload> {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const sessions = useSessions(runtimeWorkspaceId, {
    enabled: hasSession && runtimeWorkspaceId !== null,
  });
  const byId = new Map<string, SessionPayload>();
  for (const session of sessions.data ?? []) byId.set(session.id, session);
  return byId;
}

/** Measures the strip and shares its width among the tabs (UT-030 rules in `tab-width`). */
function useTabWidths(tabs: readonly OsTabModel[]) {
  // A callback ref held in state: the bar model carries no ref object read during render.
  const [strip, stripRef] = useState<HTMLDivElement | null>(null);
  const [stripWidth, setStripWidth] = useState<number | null>(null);
  useEffect(() => {
    if (strip === null) return;
    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (entry) setStripWidth(entry.contentRect.width);
    });
    observer.observe(strip);
    return () => observer.disconnect();
  }, [strip]);
  const pinned = tabs.filter(model => model.tab.pinned);
  const unpinned = tabs.filter(model => !model.tab.pinned);
  const activeIndex = unpinned.findIndex(model => model.active);
  const allocation: TabWidthAllocation | null =
    stripWidth === null
      ? null
      : allocateTabWidths({
          pinnedWidths: pinned.map(model => pinnedTabWidth(model.members.length)),
          unpinnedCount: unpinned.length,
          activeIndex: activeIndex === -1 ? null : activeIndex,
          available: stripWidth - NEW_TAB_BUTTON_WIDTH - TAB_GAP,
          gap: TAB_GAP,
        });
  const widthByTabId = new Map<string, number>();
  unpinned.forEach((model, index) => {
    const width = allocation?.widths[index];
    if (width !== undefined) widthByTabId.set(model.tab.id, width);
  });
  return { stripRef, widthByTabId, overflow: allocation?.overflow ?? false };
}

/**
 * The tab bar's view model: the active desktop's tabs, their widths against the measured strip,
 * session facts for titles and signals, and every tab command the bar and its menu run.
 */
export function useOsTabBar() {
  const { manager, coordinator } = useOsShell();
  const tabs = useOsTabModels();
  const hasSession = tabs.some(model => model.members.some(member => member.app === "session"));
  const sessionById = useTabSessions(hasSession);
  const widths = useTabWidths(tabs);
  const shortcutLabels = useDesktop(state => {
    const effective = state.windowManagerConfig?.effectiveShortcuts;
    const platform = typeof navigator === "undefined" ? "" : navigator.platform;
    return {
      close: shortcutActionLabel(effective, "window.close", platform) ?? undefined,
      newTab: shortcutActionLabel(effective, "window.tab.new", platform) ?? undefined,
      reopen: shortcutActionLabel(effective, "window.tab.reopen", platform) ?? undefined,
    };
  }, shallowEqual);
  // Reopen is offered only while the closed list holds an entry (US-006.EC-4).
  const canReopen = useDesktop(state => (state.snapshot?.closedEntryCount ?? 0) > 0);
  const activeLeadId = tabs.find(model => model.active)?.leadId ?? null;
  const modelOf = (tabId: string) => tabs.find(model => model.tab.id === tabId);

  return {
    tabs,
    sessionById,
    shortcutLabels,
    canReopen,
    ...widths,
    activate: (tabId: string) => {
      const model = modelOf(tabId);
      if (model) void coordinator.userFocus(model.leadId);
    },
    /** Closes the whole tab: its one window through the operator guard, or every split member. */
    close: (tabId: string) => {
      const model = modelOf(tabId);
      if (model === undefined) return;
      if (model.members.length === 1) void coordinator.userClose(model.leadId);
      else void manager.closeWindowScoped(model.leadId, "group");
    },
    closeScoped: (tabId: string, scope: Exclude<OsCloseScope, "tab" | "group">) => {
      const model = modelOf(tabId);
      if (model) void manager.closeWindowScoped(model.leadId, scope);
    },
    pin: (tabId: string, pinned: boolean) => {
      const model = modelOf(tabId);
      if (model) manager.pinWindow(model.leadId, pinned);
    },
    moveToDesktop: (tabId: string, desktopId: string) => {
      const model = modelOf(tabId);
      if (model) manager.moveWindowToDesktop(model.leadId, desktopId);
    },
    /** "Move left" / "Move right": one step within the tab's pinned or unpinned range. */
    moveTab: (tabId: string, direction: "left" | "right") => {
      const model = modelOf(tabId);
      if (model) manager.moveTab(model.leadId, direction);
    },
    /** "Split with ▸": the chosen tab joins this one as a split, on the right. */
    splitWith: (tabId: string, chosenTabId: string) => manager.splitWithTab(tabId, chosenTabId),
    /** "Arrange ▸": one split tab from this tab's windows and one window of every other tab. */
    arrange: (tabId: string, preset: OsArrangePreset) => {
      const model = modelOf(tabId);
      if (model) manager.arrangeLayout(model.leadId, preset);
    },
    /** "Balance sizes": evens every split of the tab. */
    balance: (tabId: string) => manager.balanceLayout({ tabId }),
    reopen: () => void manager.reopenWindow(),
    /** Opens a New Tab right after the active tab and raises its destination picker. */
    openNewTab: () => {
      void coordinator
        .userOpen({ app: "new-tab", ...(activeLeadId ? { afterWindowId: activeLeadId } : {}) })
        .then(windowId => {
          if (windowId !== null) {
            windowManagerStore.trigger.paletteIntentRequested({
              intent: { kind: "destination", windowId },
            });
          }
        })
        .catch(error => console.error("Failed to open a new tab", error));
    },
  };
}

/** The bar's render data and commands; the strip's callback ref travels apart from it. */
export type OsTabBarModel = Omit<ReturnType<typeof useOsTabBar>, "stripRef">;
