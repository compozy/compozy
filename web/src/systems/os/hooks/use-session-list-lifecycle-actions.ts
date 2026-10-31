import { useSessionLifecycleActions } from "@/systems/session";

import { useOsShell } from "./use-os-shell";

/** How a list action took a session off the list. */
export type SessionListRemoval = "archive" | "later" | "delete";

/**
 * Retire deleted list targets before their detail caches can refetch a 404. `onRemoved`
 * hears each session this host archived, moved to Later, or deleted, before a deleted
 * one's windows retire.
 */
export function useSessionListLifecycleActions(
  options: { onRemoved?: (session: { id: string }, action: SessionListRemoval) => void } = {}
) {
  const { coordinator, manager } = useOsShell();
  const { onRemoved } = options;
  return useSessionLifecycleActions({
    onArchiveSuccess: session => onRemoved?.(session, "archive"),
    onLaterSuccess: session => onRemoved?.(session, "later"),
    onDeleteSuccess: async session => {
      onRemoved?.(session, "delete");
      const windows = Object.values(manager.getState().windows);
      // The runtime serializes commands against fresh layout revisions; await every outcome.
      await Promise.all(
        windows.flatMap(window =>
          window.app === "session" && window.instanceKey === session.id
            ? [coordinator.userRetireSession(window.id)]
            : []
        )
      );
    },
  });
}
