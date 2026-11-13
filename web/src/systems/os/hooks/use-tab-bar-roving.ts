import { useEffect, useRef, useState, type FocusEvent, type KeyboardEvent } from "react";

export interface TabRovingProps {
  ref: (node: HTMLButtonElement | null) => void;
  tabIndex: 0 | -1;
  onFocus: () => void;
  onBlur: (event: FocusEvent<HTMLButtonElement>) => void;
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
}

/** Whether `node` sits inside the tablist that holds `tab`: another tab or a tab's close button. */
function withinTablist(tab: HTMLElement, node: Element | null): boolean {
  return node !== null && (tab.closest('[role="tablist"]')?.contains(node) ?? false);
}

/** A tab's own menu takes focus while open; focus there still belongs to the bar. */
function withinMenu(node: Element | null): boolean {
  return node?.closest('[role="menu"]') != null;
}

/**
 * Roving tabindex over the bar's tabs: one tab stop, arrows/Home/End move focus, Enter/Space
 * activate (manual activation). The stop is the tab focused inside the bar while focus travels
 * there, and the active tab whenever focus is elsewhere. When the focused tab closes, focus moves
 * to the tab that becomes active, or to the New tab button when no tab is left.
 */
export function useTabBarRoving(tabIds: readonly string[], activeTabId: string | null) {
  const tabRefs = new Map<string, HTMLButtonElement>();
  const [rovingId, setRovingId] = useState<string>();
  // The tab focus last rested on while it stayed in the bar or the tab's menu; null once it left.
  const lastFocusedId = useRef<string | null>(null);
  const newTabButton = useRef<HTMLButtonElement | null>(null);
  const present = new Set(tabIds);
  const stopId = rovingId !== undefined && present.has(rovingId) ? rovingId : activeTabId;
  const tabStopId = stopId !== null && present.has(stopId) ? stopId : (tabIds[0] ?? null);

  useEffect(() => {
    const closedId = lastFocusedId.current;
    if (closedId === null || present.has(closedId)) return;
    lastFocusedId.current = null;
    // Hand focus on only when it fell to the document with the closed tab.
    const focused = document.activeElement;
    if (focused !== null && focused !== document.body) return;
    const next = activeTabId === null ? undefined : tabRefs.get(activeTabId);
    (next ?? newTabButton.current)?.focus();
  });

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>, tabId: string) => {
    const position = tabIds.indexOf(tabId);
    if (position < 0 || tabIds.length === 0) return;
    // Modified arrows belong to the shell's shortcuts (directional focus, desktop switches).
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    let next: number | null = null;
    if (event.key === "ArrowLeft") next = (position - 1 + tabIds.length) % tabIds.length;
    if (event.key === "ArrowRight") next = (position + 1) % tabIds.length;
    if (event.key === "Home") next = 0;
    if (event.key === "End") next = tabIds.length - 1;
    if (next === null) return;
    event.preventDefault();
    const target = tabIds[next]!;
    setRovingId(target);
    tabRefs.get(target)?.focus();
  };

  const handleBlur = (event: FocusEvent<HTMLButtonElement>) => {
    const destination = event.relatedTarget instanceof Element ? event.relatedTarget : null;
    if (withinTablist(event.currentTarget, destination)) return;
    // Focus left the tabs: the stop returns to the active tab.
    setRovingId(undefined);
    // No destination: the window lost focus, or the tab itself is being removed; keep the record
    // so a closing tab can still hand focus on.
    if (destination !== null && !withinMenu(destination)) lastFocusedId.current = null;
  };

  const tabProps = (tabId: string): TabRovingProps => ({
    ref: node => {
      if (node) tabRefs.set(tabId, node);
      else tabRefs.delete(tabId);
    },
    tabIndex: tabId === tabStopId ? 0 : -1,
    onFocus: () => {
      lastFocusedId.current = tabId;
      setRovingId(tabId);
    },
    onBlur: handleBlur,
    onKeyDown: event => handleKeyDown(event, tabId),
  });
  const newTabRef = (node: HTMLButtonElement | null) => {
    newTabButton.current = node;
  };
  return { tabProps, newTabRef };
}
