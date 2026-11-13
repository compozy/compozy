import { useSessionLifecycleActions } from "@/systems/session";

import { useOsShell } from "./use-os-shell";

/** How a list action took a session off the list. */
export type SessionListRemoval = "archive" | "later" | "delete";

/**
 * Retire the windows of archived list targets. A delete leaves windows to the daemon, which
 * removes the deleted session's windows before the delete answers; a client retire after that
 * would only race it with a stale revision. `onRemoved` hears each session this host archived,
 * moved to Later, or deleted, before any window retires. Later keeps its windows: the
 * composer of a parked session is one of the ways back.
 */
export function useSessionListLifecycleActions(
  options: {
    windowId?: string;
    onRemoved?: (session: { id: string }, action: SessionListRemoval) => void;
  } = {}
) {
  const { coordinator, manager } = useOsShell();
  const { onRemoved } = options;
  const retireArchivedWindows = async (session: { id: string }) => {
    onRemoved?.(session, "archive");
    const windows = Object.values(manager.getState().windows);
    // The runtime serializes commands against fresh layout revisions; await every outcome.
    await Promise.all(
      windows.flatMap(window =>
        window.app === "session" && window.instanceKey === session.id
          ? [coordinator.userRetireSession(window.id)]
          : []
      )
    );
  };
  return useSessionLifecycleActions({
    archiveRequestOwnerId: options.windowId,
    onArchiveSuccess: session => retireArchivedWindows(session),
    onLaterSuccess: session => onRemoved?.(session, "later"),
    onDeleteSuccess: session => onRemoved?.(session, "delete"),
  });
}
