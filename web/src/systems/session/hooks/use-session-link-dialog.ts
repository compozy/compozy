import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

interface SessionLinkDialogContext {
  /** The session whose link dialog is showing. Transient, never persisted. */
  sessionId: string | null;
}

const sessionLinkDialogLogic = createStoreLogic({
  context: (): SessionLinkDialogContext => ({ sessionId: null }),
  on: {
    opened: (context, event: { sessionId: string }) => ({
      ...context,
      sessionId: event.sessionId,
    }),
    closed: context => ({ ...context, sessionId: null }),
  },
});

export const sessionLinkDialogStore = sessionLinkDialogLogic.createStore();

/**
 * Opens one session's link dialog. Every entry calls this (list, chip menu, composer environment
 * menu, palette), so they share the one dialog the session window hosts.
 */
export function requestSessionLinkDialog(sessionId: string): void {
  if (sessionId) sessionLinkDialogStore.trigger.opened({ sessionId });
}

export function useSessionLinkDialogOpen(sessionId: string): boolean {
  return useSelector(sessionLinkDialogStore, snapshot => snapshot.context.sessionId === sessionId);
}

export function closeSessionLinkDialog(): void {
  sessionLinkDialogStore.trigger.closed();
}
