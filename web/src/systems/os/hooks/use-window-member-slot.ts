import { type TopbarSlotStore, type TopbarSlotValue } from "@compozy/ui";
import * as React from "react";

const EMPTY_SLOT: TopbarSlotValue | null = null;
const nullSlot = () => EMPTY_SLOT;

/** Subscribes a deck segment to the slot published by its own window surface. */
export function useWindowMemberSlot(store: TopbarSlotStore | undefined): TopbarSlotValue | null {
  return React.useSyncExternalStore(
    store?.subscribe ?? (() => () => undefined),
    store?.getSnapshot ?? nullSlot,
    store?.getSnapshot ?? nullSlot
  );
}
