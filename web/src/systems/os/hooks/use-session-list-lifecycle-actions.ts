import { useSessionLifecycleActions } from "@/systems/session";

import { useOsShell } from "./use-os-shell";

/** Retire deleted list targets before their detail caches can refetch a 404. */
export function useSessionListLifecycleActions() {
  const { coordinator, manager } = useOsShell();
  return useSessionLifecycleActions({
    onDeleteSuccess: async session => {
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
