import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

interface SessionParentPickerContext {
  /** The session choosing a parent. Transient, never persisted. */
  sessionId: string | null;
}

const sessionParentPickerLogic = createStoreLogic({
  context: (): SessionParentPickerContext => ({ sessionId: null }),
  on: {
    opened: (_context, event: { sessionId: string }) => ({ sessionId: event.sessionId }),
    closed: () => ({ sessionId: null }),
  },
});

export const sessionParentPickerStore = sessionParentPickerLogic.createStore();

/**
 * Opens Move under… for one session. The row menu, its context menu and the
 * command palette all call this, so they share the one picker the shell hosts.
 */
export function requestSessionParentPicker(sessionId: string): void {
  if (sessionId) sessionParentPickerStore.trigger.opened({ sessionId });
}

export function closeSessionParentPicker(): void {
  sessionParentPickerStore.trigger.closed();
}

/** The session whose picker is open, or `null`. */
export function useSessionParentPickerRequest(): string | null {
  return useSelector(sessionParentPickerStore, snapshot => snapshot.context.sessionId);
}
