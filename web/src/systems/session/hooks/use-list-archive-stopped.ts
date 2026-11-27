import type { ComputerSessionsSource } from "./use-computer-sessions-sources";
import { USER_SCOPE_SELECTOR } from "@/systems/workspace";

import type { ArchiveStoppedScope } from "../adapters/session-archive-api";
import { useArchiveStopped } from "./use-archive-stopped";
import type { SessionListViewModel } from "./use-session-list-view";

/**
 * "Archive stopped" for the active list of one profile: one project, or every project under
 * All projects. The archive and Later shelves, the User scope and All profiles have none.
 */
export function useListArchiveStopped(
  view: SessionListViewModel,
  options: { disabled?: boolean; source?: ComputerSessionsSource | null } = {}
) {
  const source = options.source;
  const names = source?.profiles?.filter(profile => !profile.archived);
  const profile = source ? (names?.length === 1 ? names[0]!.name : null) : view.scopeLabel;
  const stopped = useArchiveStopped(profile ?? "", source?.key ?? view.computerId);
  const matched = source?.matchedWorkspaceIds;
  const workspaceId = source
    ? matched?.length === 1
      ? matched[0]
      : undefined
    : view.focusedWorkspaceId;
  const scope: ArchiveStoppedScope | null = view.allProjects
    ? { kind: "all" }
    : workspaceId && workspaceId !== USER_SCOPE_SELECTOR
      ? { kind: "project", workspaceId }
      : null;
  if (
    options.disabled ||
    (source && !source.canOperate) ||
    profile === null ||
    scope === null ||
    view.shelf !== "active"
  ) {
    return undefined;
  }
  return {
    label: scope.kind === "all" ? "Archive stopped in all projects" : "Archive stopped",
    running: stopped.running(scope),
    onSelect: () => stopped.archiveStopped(scope),
  };
}
