import { Button } from "@compozy/ui";

import { USER_SCOPE_SELECTOR } from "@/systems/workspace";

import type { ArchiveStoppedScope } from "../../adapters/session-archive-api";
import { useArchiveStopped } from "../../hooks/use-archive-stopped";
import type { useSessionListBulkSelection } from "../../hooks/use-session-list-bulk-selection";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import { AlwaysOnCreateMenu } from "../always-on/always-on-create-menu";
import { SessionListSelectionBar } from "./session-list-selection-bar";
import { SessionListToolbar } from "./session-list-toolbar";

/** The list's toolbar, or the selection bar while a bulk selection is open. */
export function SessionListControls({
  bulk,
  view,
  onNewSession,
  onNewFolder,
  newFolderUnavailable,
  testIdPrefix,
}: {
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  view: SessionListViewModel;
  onNewSession: () => void;
  onNewFolder?: () => void;
  newFolderUnavailable: string | null;
  testIdPrefix: string;
}) {
  const archiveStopped = useListArchiveStopped(view);
  return bulk.active ? (
    <SessionListSelectionBar {...bulk.bar} testIdPrefix={testIdPrefix} />
  ) : (
    <>
      <SessionListToolbar
        shelf={view.shelf}
        laterCount={view.laterCount}
        sort={view.sort}
        groupBy={view.groupBy}
        groupByOptions={view.groupByOptions}
        onShelfChange={view.setShelf}
        onSortChange={view.setSort}
        onGroupByChange={view.setGroupBy}
        showEmptyProjects={view.showEmptyProjects}
        onShowEmptyProjectsChange={view.setShowEmptyProjects}
        showAutomation={view.showAutomation}
        onShowAutomationChange={view.setShowAutomation}
        foldAll={view.foldAll}
        onFoldAllChange={view.setFoldAll}
        onNewSession={onNewSession}
        onNewFolder={onNewFolder}
        newFolderUnavailable={newFolderUnavailable}
        createMenu={
          <AlwaysOnCreateMenu
            newFolderUnavailable={newFolderUnavailable}
            onNewFolder={onNewFolder}
            onNewSession={onNewSession}
            alwaysOn={
              view.focusedWorkspaceId
                ? { place: view.focusedWorkspaceId, name: view.focusedScopeName }
                : null
            }
            testId={testIdPrefix}
            variant="quiet"
          />
        }
        archiveStopped={archiveStopped}
        testIdPrefix={testIdPrefix}
      />
      {view.sortNote ? (
        <p className="px-3 pb-1 text-micro text-subtle" data-testid={`${testIdPrefix}-sort-note`}>
          {view.sortNote}
        </p>
      ) : null}
    </>
  );
}

/**
 * "Archive stopped" for the active list of one profile: one project, or every project under
 * All projects. The archive and Later shelves, the User scope and All profiles have none.
 */
function useListArchiveStopped(view: SessionListViewModel) {
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

/** The flat catalog's paging row: Previous, Next and a retry for a failed page. */
export function SessionListCatalogNavigation({
  catalog,
}: {
  catalog: NonNullable<SessionListViewModel["catalog"]>;
}) {
  return (
    <div className="flex items-center justify-between px-2 py-2">
      {catalog.paging ? <span role="status">Loading sessions…</span> : null}
      {catalog.previous ? (
        <Button variant="ghost" size="sm" disabled={catalog.paging} onClick={catalog.previousPage}>
          Previous sessions
        </Button>
      ) : (
        <span />
      )}
      {catalog.next ? (
        <Button variant="ghost" size="sm" disabled={catalog.paging} onClick={catalog.nextPage}>
          Next sessions
        </Button>
      ) : null}
      {/* An empty failed list already states the failure with its one Retry above. */}
      {catalog.failed && catalog.sessions.length > 0 ? (
        <Button variant="ghost" size="sm" onClick={catalog.retry}>
          Retry loading sessions
        </Button>
      ) : null}
    </div>
  );
}
