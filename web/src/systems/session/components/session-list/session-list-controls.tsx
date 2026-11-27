import { Button } from "@compozy/ui";

import { useListArchiveStopped } from "../../hooks/use-list-archive-stopped";
import type { useSessionListBulkSelection } from "../../hooks/use-session-list-bulk-selection";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionViewOptionsMenuProps } from "../session-view-options-menu";
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
  computerLayout,
  laterCount,
  archiveStoppedDisabled = false,
  disableAlwaysOnCreate = false,
  testIdPrefix,
}: {
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  view: SessionListViewModel;
  onNewSession: () => void;
  onNewFolder?: () => void;
  newFolderUnavailable: string | null;
  computerLayout?: SessionViewOptionsMenuProps["computerLayout"];
  laterCount?: number;
  archiveStoppedDisabled?: boolean;
  disableAlwaysOnCreate?: boolean;
  testIdPrefix: string;
}) {
  const archiveStopped = useListArchiveStopped(view, { disabled: archiveStoppedDisabled });
  return bulk.active ? (
    <SessionListSelectionBar {...bulk.bar} testIdPrefix={testIdPrefix} />
  ) : (
    <>
      <SessionListToolbar
        shelf={view.shelf}
        laterCount={laterCount ?? view.laterCount}
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
        computerLayout={computerLayout}
        createMenu={
          <AlwaysOnCreateMenu
            newFolderUnavailable={newFolderUnavailable}
            onNewFolder={onNewFolder}
            onNewSession={onNewSession}
            alwaysOn={
              !disableAlwaysOnCreate && view.focusedWorkspaceId
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

/** The flat catalog's paging row: Previous, Next and a retry for a failed page. */
export function SessionListCatalogNavigation({
  catalog,
}: {
  catalog: Pick<
    NonNullable<SessionListViewModel["catalog"]>,
    "paging" | "previous" | "next" | "previousPage" | "nextPage" | "failed" | "sessions" | "retry"
  >;
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
