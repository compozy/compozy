import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useEffect, useId } from "react";

interface SessionLinkDialogContext {
  /** The session whose link dialog is showing. Transient, never persisted. */
  sessionId: string | null;
  /** Mounted hosts per session, oldest first: only the oldest shows the dialog. */
  hosts: Readonly<Record<string, readonly string[]>>;
}

const sessionLinkDialogLogic = createStoreLogic({
  context: (): SessionLinkDialogContext => ({ sessionId: null, hosts: {} }),
  on: {
    opened: (context, event: { sessionId: string }) => ({
      ...context,
      sessionId: event.sessionId,
    }),
    closed: context => ({ ...context, sessionId: null }),
    hostMounted: (context, { sessionId, host }: { sessionId: string; host: string }) => ({
      ...context,
      hosts: { ...context.hosts, [sessionId]: [...(context.hosts[sessionId] ?? []), host] },
    }),
    hostUnmounted: (context, { sessionId, host }: { sessionId: string; host: string }) => ({
      ...context,
      hosts: {
        ...context.hosts,
        [sessionId]: (context.hosts[sessionId] ?? []).filter(entry => entry !== host),
      },
    }),
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

/** Two windows can show one session; a request still opens one dialog. */
export function useSessionLinkDialogOpen(sessionId: string): boolean {
  const host = useId();
  useEffect(() => {
    sessionLinkDialogStore.trigger.hostMounted({ sessionId, host });
    return () => sessionLinkDialogStore.trigger.hostUnmounted({ sessionId, host });
  }, [sessionId, host]);
  return useSelector(
    sessionLinkDialogStore,
    ({ context }) => context.sessionId === sessionId && context.hosts[sessionId]?.[0] === host
  );
}

export function closeSessionLinkDialog(): void {
  sessionLinkDialogStore.trigger.closed();
}
