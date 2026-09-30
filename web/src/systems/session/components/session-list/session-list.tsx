import { useState } from "react";

import { WifiOff } from "lucide-react";

import { Alert, AlertDescription, Button, SearchInput } from "@compozy/ui";

import { buildSessionListThreads, countThreadSessions } from "../../lib/session-list-threads";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { emptyForScope } from "@/systems/profiles";

import { useSessionListBulkSelection } from "../../hooks/use-session-list-bulk-selection";
import { SessionListSelectionBar } from "./session-list-selection-bar";
import { SessionListThreadList } from "./session-list-thread-list";
import { SessionListToolbar } from "./session-list-toolbar";
import { SessionListWorkspaceGroups } from "./session-list-workspace-groups";

export interface SessionListProps {
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedThreadIds: readonly string[];
  currentSessionId?: string;
  /** Breadth, order, and the widened per-workspace groups. */
  view: SessionListViewModel;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onNewSession: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Rendered above the controls (the modal's eyebrow + close chrome). */
  header?: (visibleCount: number) => React.ReactNode;
}

/**
 * Say what is empty AND for whom: an operator in Marketing needs to know the
 * list is empty in Marketing, not on the machine (US-009.EC-3).
 */
function emptySessionListMessage(filtering: boolean, view: SessionListViewModel): string {
  if (filtering) return `${emptyForScope("matching sessions", view.scopeLabel)}.`;
  if (view.archived) return `${emptyForScope("archived sessions", view.scopeLabel)}.`;
  return `${emptyForScope("sessions", view.scopeLabel)}.`;
}

/**
 * Shared sessions catalog body: this workspace, or every workspace.
 *
 * The narrow breadth pages the active workspace catalog as
 * provenance threads; `All workspaces` widens to every workspace, grouped and
 * labelled, so no workspace can stall unnoticed. Ordering is served by the
 * daemon for the operator's chosen sort — this component renders the order it
 * receives rather than re-deciding it.
 *
 * Loaded parents retain their descendants as threads. A parent outside the
 * current page leaves its child visible as a root rather than hiding it.
 */
export function SessionList({
  sessions,
  disconnected,
  collapsedThreadIds,
  currentSessionId,
  view,
  onToggleThread,
  onSelectSession,
  onNewSession,
  sessionActions,
  testIdPrefix,
  header,
}: SessionListProps) {
  const [filter, setFilter] = useState("");
  const remoteSearch = view.search !== undefined;
  const normalizedFilter = (view.search ?? filter).trim().toLocaleLowerCase();
  const threads = buildSessionListThreads(sessions, remoteSearch ? "" : normalizedFilter);
  const visibleCount = countThreadSessions(threads);
  const collapsedThreads = new Set(collapsedThreadIds);
  const bulk = useSessionListBulkSelection({
    scope: view.scope,
    archived: view.archived,
    sessions,
    threads,
    collapsedThreads,
    sessionActions,
  });

  return (
    <div
      className="flex min-h-0 flex-1 flex-col"
      data-testid={`${testIdPrefix}-content`}
      onKeyDown={bulk.onKeyDown}
    >
      {header?.(visibleCount)}
      <SessionListControls
        bulk={bulk}
        view={view}
        onNewSession={onNewSession}
        testIdPrefix={testIdPrefix}
      />
      <div className="px-3 pb-1.5">
        <SearchInput
          value={view.search ?? filter}
          onChange={view.setSearch ?? setFilter}
          placeholder="Filter sessions…"
          aria-label="Filter sessions"
          containerClassName="min-w-0"
        />
      </div>
      {disconnected ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <WifiOff aria-hidden="true" />
          <AlertDescription>
            Can&apos;t get session updates right now. The sessions below may be out of date.
          </AlertDescription>
        </Alert>
      ) : null}
      <SessionListBody
        view={view}
        threads={threads}
        collapsedThreads={collapsedThreads}
        currentSessionId={currentSessionId}
        normalizedFilter={normalizedFilter}
        onToggleThread={onToggleThread}
        onSelectSession={onSelectSession}
        selection={bulk.rowSelection}
        sessionActions={sessionActions}
        testIdPrefix={testIdPrefix}
      />
    </div>
  );
}

function SessionListControls({
  bulk,
  view,
  onNewSession,
  testIdPrefix,
}: {
  bulk: ReturnType<typeof useSessionListBulkSelection>;
  view: SessionListViewModel;
  onNewSession: SessionListProps["onNewSession"];
  testIdPrefix: string;
}) {
  const allWorkspaces = view.scope === "all-workspaces";
  return bulk.active ? (
    <SessionListSelectionBar {...bulk.bar} testIdPrefix={testIdPrefix} />
  ) : (
    <SessionListToolbar
      allWorkspaces={allWorkspaces}
      archived={view.archived}
      sort={view.sort}
      disabled={view.saving}
      onAllWorkspacesChange={next => view.setScope(next ? "all-workspaces" : "workspace")}
      onArchivedChange={view.setArchived}
      onSortChange={view.setSort}
      onNewSession={onNewSession}
      testIdPrefix={testIdPrefix}
    />
  );
}

function SessionListBody({
  view,
  threads,
  collapsedThreads,
  currentSessionId,
  normalizedFilter,
  onToggleThread,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
}: Pick<
  SessionListProps,
  | "view"
  | "currentSessionId"
  | "onToggleThread"
  | "onSelectSession"
  | "sessionActions"
  | "testIdPrefix"
> & {
  threads: ReturnType<typeof buildSessionListThreads>;
  collapsedThreads: ReadonlySet<string>;
  normalizedFilter: string;
  selection: ReturnType<typeof useSessionListBulkSelection>["rowSelection"];
}) {
  const allWorkspaces = view.scope === "all-workspaces";
  const ownerOf = view.aggregate ? view.ownerOf : undefined;
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
      data-scope={view.scope}
      aria-busy={view.catalog?.paging || undefined}
    >
      {allWorkspaces ? (
        <SessionListWorkspaceGroups
          groups={view.workspaceGroups}
          collapsedWorkspaceIds={view.collapsedWorkspaceIds}
          currentSessionId={currentSessionId}
          ownerOf={ownerOf}
          scopeLabel={view.scopeLabel}
          archived={view.archived}
          onToggleWorkspace={view.toggleWorkspace}
          onSelectSession={session => {
            if (!view.catalog?.paging) onSelectSession(session);
          }}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
        />
      ) : (
        <SessionListThreadList
          threads={threads}
          collapsedThreads={collapsedThreads}
          currentSessionId={currentSessionId}
          ownerOf={ownerOf}
          onToggleThread={onToggleThread}
          onSelectSession={session => {
            if (!view.catalog?.paging) onSelectSession(session);
          }}
          selection={selection}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
          emptyMessage={emptySessionListMessage(normalizedFilter !== "", view)}
        />
      )}
      {!allWorkspaces && view.catalog ? (
        <SessionListCatalogNavigation catalog={view.catalog} />
      ) : null}
    </div>
  );
}

function SessionListCatalogNavigation({
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
      {catalog.failed ? (
        <Button variant="ghost" size="sm" onClick={catalog.retry}>
          Retry loading sessions
        </Button>
      ) : null}
    </div>
  );
}
