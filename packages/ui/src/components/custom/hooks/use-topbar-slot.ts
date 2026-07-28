import * as React from "react";

/** Parent crumb in a window-local drill-in trail (leaf is `crumb` / Topbar title). */
export interface TopbarCrumb {
  /** Stable list key — required so path collapse/reorder never remounts the wrong crumb. */
  id: string;
  label: React.ReactNode;
  onSelect: () => void;
}

export interface TopbarSlotValue {
  /**
   * Override for the current route / leaf title. Lets routes that resolve
   * identity from loader data push it as a live node.
   */
  crumb?: React.ReactNode;
  /** Quiet identity glyph at window root (hidden when drill-in crumbs/back are set). */
  glyph?: React.ReactNode;
  /** Icon well by default; `state` renders a bare live-state mark for document windows. */
  glyphPresentation?: "icon" | "state";
  /** Optional mono count beside the title at root. */
  count?: React.ReactNode;
  /** Live status chip in the head trail (state-colored). */
  status?: React.ReactNode;
  /**
   * Window-local parent crumbs for in-place navigation. When set (or `onBack`
   * is set), the head uses the drill-in variant — back chevron replaces the glyph.
   */
  crumbs?: readonly TopbarCrumb[];
  /** Pop one location level (drill-in back affordance). */
  onBack?: () => void;
  /** Action buttons in the trailing zone (≤1 primary + companions). */
  actions?: React.ReactNode;
  /** Overflow menu rendered last in the trailing zone. */
  overflow?: React.ReactNode;
  /**
   * Peer route tabs (`RouteNav`) rendered in the 44px head immediately after
   * identity. Absent on drill-in and on routes without sibling views.
   */
  nav?: React.ReactNode;
  /** Optional 38px context-strip content (filters / search / view toggles). */
  toolbar?: React.ReactNode;
}

interface TopbarSlotPublisher {
  publishSlot: (owner: object, slot: TopbarSlotValue | null) => void;
  clearSlot: (owner: object) => void;
}

interface TopbarSlotStore extends TopbarSlotPublisher {
  getSnapshot: () => TopbarSlotValue | null;
  subscribe: (listener: () => void) => () => void;
}

export const TopbarSlotSettersContext = React.createContext<TopbarSlotPublisher | null>(null);

export const TopbarSlotContext = React.createContext<TopbarSlotStore | null>(null);

/**
 * Reference equality per zone. ReactNode contents are compared by identity —
 * a publisher rendering fresh JSX republishes, which is required so replaced
 * action handlers reach the topbar.
 */
function isSameTopbarSlot(a: TopbarSlotValue | null, b: TopbarSlotValue | null): boolean {
  if (a === b) return true;
  if (a == null || b == null) return false;
  return (
    a.crumb === b.crumb &&
    a.glyph === b.glyph &&
    a.glyphPresentation === b.glyphPresentation &&
    a.count === b.count &&
    a.status === b.status &&
    a.crumbs === b.crumbs &&
    a.onBack === b.onBack &&
    a.actions === b.actions &&
    a.overflow === b.overflow &&
    a.nav === b.nav &&
    a.toolbar === b.toolbar
  );
}

export function createTopbarSlotStore(): TopbarSlotStore {
  let active: { owner: object; slot: TopbarSlotValue | null } | null = null;
  const listeners = new Set<() => void>();
  const emit = () => {
    for (const listener of listeners) listener();
  };

  return {
    getSnapshot: () => active?.slot ?? null,
    subscribe: listener => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    publishSlot: (owner, slot) => {
      if (active?.owner === owner && isSameTopbarSlot(active.slot, slot)) return;
      active = { owner, slot };
      emit();
    },
    clearSlot: owner => {
      if (active?.owner !== owner) return;
      active = null;
      emit();
    },
  };
}

const subscribeToNothing = () => () => undefined;
const getNullSnapshot = () => null;

/**
 * Pushes a topbar slot for the lifetime of the calling component.
 */
export function useTopbarSlot(slot: TopbarSlotValue | null): void {
  const setters = React.use(TopbarSlotSettersContext);
  const [owner] = React.useState<object>(() => ({}));
  React.useLayoutEffect(() => {
    if (!setters) return;
    if (slot === null) {
      setters.clearSlot(owner);
      return;
    }
    setters.publishSlot(owner, slot);
  }, [owner, setters, slot]);
  React.useEffect(() => {
    if (!setters) return;
    return () => setters.clearSlot(owner);
  }, [owner, setters]);
}

export function useTopbarSlotValue(): TopbarSlotValue | null {
  const store = React.use(TopbarSlotContext);
  return React.useSyncExternalStore(
    store?.subscribe ?? subscribeToNothing,
    store?.getSnapshot ?? getNullSnapshot,
    store?.getSnapshot ?? getNullSnapshot
  );
}
