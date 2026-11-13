import { useState, type KeyboardEvent } from "react";

/** What a tab needs to take part in the bar's single tab stop. */
export interface TabRovingProps {
  ref: (node: HTMLButtonElement | null) => void;
  tabIndex: 0 | -1;
  onFocus: () => void;
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
}

/**
 * The tab bar's roving tabindex (`_spec.md` Accessibility): one tab stop, on the last focused tab
 * while it is still in the bar, else the active tab. Left and Right move focus and wrap, Home and
 * End jump to the ends. Activation stays manual: Enter or Space presses the focused tab.
 */
export function useTabBarRoving(tabIds: readonly string[], activeTabId: string | null) {
  const tabRefs = new Map<string, HTMLButtonElement>();
  const [rovingId, setRovingId] = useState<string>();
  const present = new Set(tabIds);
  const stopId = rovingId !== undefined && present.has(rovingId) ? rovingId : activeTabId;
  const tabStopId = stopId !== null && present.has(stopId) ? stopId : (tabIds[0] ?? null);

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

  const tabProps = (tabId: string): TabRovingProps => ({
    ref: node => {
      if (node) tabRefs.set(tabId, node);
      else tabRefs.delete(tabId);
    },
    tabIndex: tabId === tabStopId ? 0 : -1,
    onFocus: () => setRovingId(tabId),
    onKeyDown: event => handleKeyDown(event, tabId),
  });

  return { tabProps };
}
