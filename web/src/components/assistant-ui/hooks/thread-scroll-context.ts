import { createContext, use } from "react";

import type { threadScrollLogic } from "./thread-scroll-store";

export type ThreadScrollStore = ReturnType<typeof threadScrollLogic.createStore>;

/**
 * The transcript's one scroll owner, shared by the viewport controller and the
 * rows that change height on disclosure (folds, groups): a toggle tells the
 * owner to hold end-maintenance for two frames so opening never yanks (ADR-007).
 * Absent outside a thread viewport (stories, read-only rows) — toggles are then
 * a no-op for scrolling.
 */
export const ThreadScrollStoreContext = createContext<ThreadScrollStore | null>(null);

export function useOptionalThreadScrollStore(): ThreadScrollStore | null {
  return use(ThreadScrollStoreContext);
}

/** Notify the scroll owner that a disclosure changed a row's height. */
export function notifyDisclosureToggled(store: ThreadScrollStore | null): void {
  store?.trigger.disclosureToggled({});
}
