import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
} from "react";

export interface TabRovingProps {
  ref: (node: HTMLButtonElement | null) => void;
  tabIndex: 0 | -1;
  onFocus: () => void;
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
}

/**
 * The bar's per-tab roving handlers, taking the tab they act for. They keep their identity across
 * renders, so a tab re-renders only when its own stop changes.
 */
export interface TabRovingHandlers {
  register: (tabId: string, node: HTMLButtonElement | null) => void;
  focus: (tabId: string) => void;
  keyDown: (event: KeyboardEvent<HTMLButtonElement>, tabId: string) => void;
}

/** Focus handlers for the tablist itself: they see focus on any tab and on any close button. */
export interface TablistFocusProps {
  onFocus: (event: FocusEvent<HTMLElement>) => void;
  onBlur: (event: FocusEvent<HTMLElement>) => void;
}

/** How long a closing tab menu may keep focus before the bar stops waiting to hand it on. */
const MENU_WAIT_MS = 16;
const MENU_WAIT_LIMIT = 30;

/** A tab's own menu takes focus while open; focus there still belongs to the bar. */
function withinMenu(node: Element | null): boolean {
  return node?.closest('[role="menu"]') != null;
}

/** The tab whose plate holds `node`: the tab button or its close button. */
function plateTabId(node: EventTarget | null): string | null {
  if (!(node instanceof Element)) return null;
  return node.closest<HTMLElement>("[data-tab-id]")?.dataset.tabId ?? null;
}

/**
 * Roving tabindex over the bar's tabs: one tab stop, arrows/Home/End move focus, Enter/Space
 * activate (manual activation). The stop is the tab focused inside the bar while focus travels
 * there, and the active tab whenever focus is elsewhere. When the tab holding focus (its button,
 * its close button or its open menu) closes, focus moves to the tab that becomes active, to the
 * first tab when none is active, or to the New tab button when no tab is left. Focus that already
 * rests on something else, such as a window, a dialog or the palette, is never taken.
 */
export function useTabBarRoving(tabIds: readonly string[], activeTabId: string | null) {
  const tabRefs = useRef(new Map<string, HTMLButtonElement>());
  // The bar's order as last rendered, for the arrow keys: read when a key is pressed.
  const tabOrder = useRef(tabIds);
  useLayoutEffect(() => {
    tabOrder.current = tabIds;
  });
  const [rovingId, setRovingId] = useState<string>();
  // The tab whose plate or menu held focus last; null once focus left the bar for anything else.
  const lastFocusedId = useRef<string | null>(null);
  const newTabButton = useRef<HTMLButtonElement | null>(null);
  const present = new Set(tabIds);
  const stopId = rovingId !== undefined && present.has(rovingId) ? rovingId : activeTabId;
  const tabStopId = stopId !== null && present.has(stopId) ? stopId : (tabIds[0] ?? null);

  useEffect(() => {
    const closedId = lastFocusedId.current;
    if (closedId === null || present.has(closedId)) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let waits = 0;
    const handOff = () => {
      const focused = document.activeElement;
      // The closed tab's menu can outlive it for a moment; decide once it lets focus go.
      if (withinMenu(focused) && waits < MENU_WAIT_LIMIT) {
        waits += 1;
        timer = setTimeout(handOff, MENU_WAIT_MS);
        return;
      }
      lastFocusedId.current = null;
      // Hand focus on only when it fell to the document with the closed tab.
      if (focused !== null && focused !== document.body) return;
      const nextId = activeTabId !== null && present.has(activeTabId) ? activeTabId : tabIds[0];
      const target = nextId === undefined ? undefined : tabRefs.current.get(nextId);
      (target ?? newTabButton.current)?.focus();
    };
    handOff();
    return () => clearTimeout(timer);
  });

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>, tabId: string) => {
    const order = tabOrder.current;
    const position = order.indexOf(tabId);
    if (position < 0 || order.length === 0) return;
    // Modified arrows belong to the shell's shortcuts (directional focus, desktop switches).
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    let next: number | null = null;
    if (event.key === "ArrowLeft") next = (position - 1 + order.length) % order.length;
    if (event.key === "ArrowRight") next = (position + 1) % order.length;
    if (event.key === "Home") next = 0;
    if (event.key === "End") next = order.length - 1;
    if (next === null) return;
    event.preventDefault();
    const target = order[next]!;
    setRovingId(target);
    tabRefs.current.get(target)?.focus();
  };

  const tablistProps: TablistFocusProps = {
    onFocus: event => {
      const tabId = plateTabId(event.target);
      if (tabId !== null) lastFocusedId.current = tabId;
    },
    onBlur: event => {
      const destination = event.relatedTarget instanceof Element ? event.relatedTarget : null;
      if (destination !== null && event.currentTarget.contains(destination)) return;
      // Focus left the tabs and their close buttons: the stop returns to the active tab.
      setRovingId(undefined);
      // No destination: the window lost focus, or the focused node is being removed; keep the
      // record so a closing tab can still hand focus on.
      if (destination !== null && !withinMenu(destination)) lastFocusedId.current = null;
    },
  };

  const handlers: TabRovingHandlers = {
    register: (tabId, node) => {
      if (node) tabRefs.current.set(tabId, node);
      else tabRefs.current.delete(tabId);
    },
    focus: tabId => setRovingId(tabId),
    keyDown: handleKeyDown,
  };
  const newTabRef = (node: HTMLButtonElement | null) => {
    newTabButton.current = node;
  };
  return { tabStopId, handlers, tablistProps, newTabRef };
}
