import { shallowEqual } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useEffect, useState } from "react";

import { type SessionPayload, useSessions } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import type {
  OsArrangePreset,
  OsCloseScope,
  OsDesktopRuntimeStore,
  OsWindow,
} from "../lib/os-types";
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

/** The same tab, active flag, lead and members (each window by identity): the tab shows the same. */
export function sameTabModel(left: OsTabModel, right: OsTabModel): boolean {
  return (
    left.tab === right.tab &&
    left.active === right.active &&
    left.leadId === right.leadId &&
    left.members.length === right.members.length &&
    left.members.every((member, index) => member === right.members[index])
  );
}

function sameTabModels(left: readonly OsTabModel[] | undefined, right: readonly OsTabModel[]) {
  if (left === undefined || left.length !== right.length) return false;
  return left.every((model, index) => sameTabModel(model, right[index]!));
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
  const commands = useOsTabCommands();

  return { tabs, sessionById, shortcutLabels, canReopen, ...widths, ...commands };
}

/**
 * A tab's lead window and member count as the bar names them (`useOsTabModels`), read from the
 * state a command runs against rather than the bar's last render.
 */
function tabLeadAt(
  state: OsDesktopRuntimeStore,
  tabId: string
): { leadId: string; memberCount: number } | null {
  for (const desktop of state.desktops) {
    const tab = desktop.tabs.find(entry => entry.id === tabId);
    if (tab === undefined) continue;
    const leadId = leadWindowId(tab, state.client?.focusOrder ?? []);
    const memberCount = tabWindowIds(tab).filter(id => state.windows[id] !== undefined).length;
    return leadId === null || memberCount === 0 ? null : { leadId, memberCount };
  }
  return null;
}

/** The active desktop's active tab lead: where a New Tab opens after. */
function activeLeadAt(state: OsDesktopRuntimeStore): string | null {
  const desktop = state.desktops.find(entry => entry.id === state.activeDesktopId);
  if (desktop === undefined) return null;
  const activeTabId = state.activeTabIds[desktop.id] ?? resolveActiveTabId(desktop, null);
  return activeTabId === null ? null : (tabLeadAt(state, activeTabId)?.leadId ?? null);
}

/**
 * Every tab command the bar and its menu run. Each resolves its tab when it runs, so the
 * commands keep their identity across switches and a tab whose own facts did not change does not
 * re-render with the bar.
 */
function useOsTabCommands() {
  const { manager, coordinator } = useOsShell();
  const leadOf = (tabId: string) => tabLeadAt(manager.getState(), tabId);
  return {
    activate: (tabId: string) => {
      const lead = leadOf(tabId);
      if (lead) void coordinator.userFocus(lead.leadId);
    },
    /** Closes the whole tab: its one window through the operator guard, or every split member. */
    close: (tabId: string) => {
      const lead = leadOf(tabId);
      if (lead === null) return;
      if (lead.memberCount === 1) void coordinator.userClose(lead.leadId);
      else void manager.closeWindowScoped(lead.leadId, "group");
    },
    closeScoped: (tabId: string, scope: Exclude<OsCloseScope, "tab" | "group">) => {
      const lead = leadOf(tabId);
      if (lead) void manager.closeWindowScoped(lead.leadId, scope);
    },
    pin: (tabId: string, pinned: boolean) => {
      const lead = leadOf(tabId);
      if (lead) manager.pinWindow(lead.leadId, pinned);
    },
    moveToDesktop: (tabId: string, desktopId: string) => {
      const lead = leadOf(tabId);
      if (lead) manager.moveWindowToDesktop(lead.leadId, desktopId);
    },
    /** "Move left" / "Move right": one step within the tab's pinned or unpinned range. */
    moveTab: (tabId: string, direction: "left" | "right") => {
      const lead = leadOf(tabId);
      if (lead) manager.moveTab(lead.leadId, direction);
    },
    /** "Split with ▸": the chosen tab joins this one as a split, on the right. */
    splitWith: (tabId: string, chosenTabId: string) => manager.splitWithTab(tabId, chosenTabId),
    /** "Arrange ▸": one split tab from this tab's windows and one window of every other tab. */
    arrange: (tabId: string, preset: OsArrangePreset) => {
      const lead = leadOf(tabId);
      if (lead) manager.arrangeLayout(lead.leadId, preset);
    },
    /** "Balance sizes": evens every split of the tab that can be evened without collapsing one. */
    balance: (tabId: string) => manager.balanceTab(tabId),
    reopen: () => void manager.reopenWindow(),
    /** Opens a New Tab right after the active tab and raises its destination picker. */
    openNewTab: () => {
      const activeLeadId = activeLeadAt(manager.getState());
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
