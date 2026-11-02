import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useEffect, useId } from "react";

interface SessionLinkDialogContext {
  /** The session whose link dialog is showing. Transient, never persisted. */
  sessionId: string | null;
  /** The host showing it. Picked once, so a focus change can't throw away the picks. */
  host: string | null;
}

const sessionLinkDialogLogic = createStoreLogic({
  context: (): SessionLinkDialogContext => ({ sessionId: null, host: null }),
  on: {
    opened: (context, event: { sessionId: string }) => ({
      ...context,
      sessionId: event.sessionId,
      host: null,
    }),
    claimed: (context, event: { host: string }) => ({
      ...context,
      host: context.host ?? event.host,
    }),
    released: (context, event: { host: string }) => ({
      ...context,
      host: context.host === event.host ? null : context.host,
    }),
    closed: context => ({ ...context, sessionId: null, host: null }),
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

/**
 * Two windows can show one session. The focused one takes the request, because every entry acts
 * on it, and keeps the dialog until it closes.
 */
export function useSessionLinkDialogOpen(sessionId: string, focused: boolean): boolean {
  const host = useId();
  const requested = useSelector(
    sessionLinkDialogStore,
    snapshot => snapshot.context.sessionId === sessionId
  );
  useEffect(() => {
    if (requested && focused) sessionLinkDialogStore.trigger.claimed({ host });
  }, [requested, focused, host]);
  useEffect(() => () => sessionLinkDialogStore.trigger.released({ host }), [host]);
  return useSelector(
    sessionLinkDialogStore,
    snapshot => requested && snapshot.context.host === host
  );
}

export function closeSessionLinkDialog(): void {
  sessionLinkDialogStore.trigger.closed();
}
