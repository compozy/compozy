import { useSessionLifecycleActions } from "@/systems/session";

import { useOsShell } from "./use-os-shell";

/** How a list action took a session off the list. */
export type SessionListRemoval = "archive" | "later" | "delete";

/**
 * Retire the windows of archived and deleted list targets; a delete does so before its
 * detail caches can refetch a 404. `onRemoved` hears each session this host archived, moved
 * to Later, or deleted, before its windows retire. Later keeps its windows: the composer of
 * a parked session is one of the ways back.
 */
export function useSessionListLifecycleActions(
  options: { onRemoved?: (session: { id: string }, action: SessionListRemoval) => void } = {}
) {
  const { coordinator, manager } = useOsShell();
  const { onRemoved } = options;
  const retireWindows = async (session: { id: string }, action: "archive" | "delete") => {
    onRemoved?.(session, action);
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
    onArchiveSuccess: session => retireWindows(session, "archive"),
    onLaterSuccess: session => onRemoved?.(session, "later"),
    onDeleteSuccess: session => retireWindows(session, "delete"),
  });
}
