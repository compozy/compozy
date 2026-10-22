import {
  GLOBAL_DESKTOP_WORKSPACE_ID,
  useActiveWorkspace,
  WINDOW_BINDING_SEARCH_KEY,
  windowDataScope,
  workspaceBadgeIdentity,
  type WindowDataBinding,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import { useDesktop } from "./use-desktop";

export interface WindowBinding {
  /** Provided to the window's subtree for `useWindowDataWorkspace`. */
  data: WindowDataBinding;
  /** The item's project for the chrome badge: only bound windows on the All projects desktop. */
  badge: WorkspaceBadgeIdentity | null;
}

/**
 * One window's partition and serialized data binding, plus the project badge
 * its chrome shows on the All projects desktop. Catalog windows (unbound)
 * read the aggregate and show no badge.
 */
export function useWindowBinding(windowId: string): WindowBinding {
  const partition = useDesktop(state => state.snapshot?.workspaceId ?? null);
  const binding = useDesktop(
    state => state.windows[windowId]?.route.search[WINDOW_BINDING_SEARCH_KEY]
  );
  const { registeredWorkspaces } = useActiveWorkspace();
  const scope = windowDataScope(partition, binding);
  const bound = partition === GLOBAL_DESKTOP_WORKSPACE_ID && typeof binding === "string";
  const badge =
    bound && scope !== null && scope.kind !== "all"
      ? workspaceBadgeIdentity(
          scope.kind === "user" ? "" : scope.workspaceId,
          new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]))
        )
      : null;
  return { data: { partition, binding }, badge };
}
