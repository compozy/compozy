import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { useSessionLifecycleActions } from "@/systems/session";
import { sessionComputerOf } from "@/systems/session/lib/session-row-ref";

import { useOsShell } from "./use-os-shell";

/** How a list action took a session off the list. */
export type SessionListRemoval = "archive" | "later" | "delete";

/**
 * Clear archived selections here and let the daemon clear deleted selections before the delete
 * answers. `onRemoved` reports each session this host archived, moved to Later, or deleted.
 * Later keeps its selection so the parked session's composer is still a way back.
 */
export function useSessionListLifecycleActions(
  options: {
    windowId?: string;
    onRemoved?: (session: { id: string; computerId?: ComputerId }, action: SessionListRemoval) => void;
  } = {}
) {
  const { coordinator, manager } = useOsShell();
  const { onRemoved } = options;
  const retireArchivedWindows = async (session: { id: string; computerId?: ComputerId }) => {
    onRemoved?.(session, "archive");
    const targetComputer = sessionComputerOf(session);
    const windows = Object.values(manager.getState().windows);
    // The runtime serializes commands against fresh layout revisions; await every outcome.
    await Promise.all(
      windows.flatMap(window =>
        window.app === "session" &&
        window.instanceKey === session.id &&
        normalizeComputerId(window.route.search?.computer) === targetComputer
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
