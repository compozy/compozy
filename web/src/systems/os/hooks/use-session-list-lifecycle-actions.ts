import { useSessionLifecycleActions } from "@/systems/session";

import { useOsShell } from "./use-os-shell";

/**
 * Retire deleted list targets before their detail caches can refetch a 404. `onRemoved`
 * hears each session this host archived or deleted, before a deleted one's windows retire.
 */
export function useSessionListLifecycleActions(
  options: { onRemoved?: (session: { id: string }, action: "archive" | "delete") => void } = {}
) {
  const { coordinator, manager } = useOsShell();
  const { onRemoved } = options;
  return useSessionLifecycleActions({
    onArchiveSuccess: session => onRemoved?.(session, "archive"),
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
