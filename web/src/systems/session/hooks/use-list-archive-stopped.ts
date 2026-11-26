import { USER_SCOPE_SELECTOR } from "@/systems/workspace";

import type { ArchiveStoppedScope } from "../adapters/session-archive-api";
import { useArchiveStopped } from "./use-archive-stopped";
import type { SessionListViewModel } from "./use-session-list-view";

/**
 * "Archive stopped" for the active list of one profile: one project, or every project under
 * All projects. The archive and Later shelves, the User scope and All profiles have none.
 */
export function useListArchiveStopped(view: SessionListViewModel) {
  const profile = view.scopeLabel;
  const stopped = useArchiveStopped(profile ?? "");
  const scope: ArchiveStoppedScope | null = view.allProjects
    ? { kind: "all" }
    : view.focusedWorkspaceId && view.focusedWorkspaceId !== USER_SCOPE_SELECTOR
      ? { kind: "project", workspaceId: view.focusedWorkspaceId }
      : null;
  if (profile === null || scope === null || view.shelf !== "active") return undefined;
  return {
    label: scope.kind === "all" ? "Archive stopped in all projects" : "Archive stopped",
    running: stopped.running(scope),
    onSelect: () => stopped.archiveStopped(scope),
  };
}
