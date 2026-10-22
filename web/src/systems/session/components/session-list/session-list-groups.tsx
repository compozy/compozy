import { ChevronRight, TriangleAlert } from "lucide-react";

import { Button, Icon, StatusDot, cn } from "@compozy/ui";

import { ProfileOwnerTag, emptyForScope } from "@/systems/profiles";
import { WorkspaceBadge, workspaceWorkingLabel } from "@/systems/workspace";

import { useSessionCatalog } from "../../hooks/use-session-catalog";
import { useSessionFolders } from "../../hooks/use-session-folders";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import { sessionListGroupFilters, type SessionListGroupSpec } from "../../lib/session-list-groups";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import { buildSessionListThreads } from "../../lib/session-list-threads";
import type { SessionPayload } from "../../types";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListSections } from "./session-list-sections";
import { SessionListThread } from "./session-list-thread";

interface GroupRowProps {
  view: SessionListViewModel;
  filtering: boolean;
  collapsedThreads: ReadonlySet<string>;
  currentSessionId?: string;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onNewSessionInFolder: (folderId: string) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

function GroupNote({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-6 items-center gap-2 px-3 py-1 text-micro text-subtle">
      {children}
    </div>
  );
}

/** Collapsed header signal: needs-you outranks working; an open group shows its rows instead. */
function GroupSignal({ group }: { group: SessionListGroupSpec }) {
  if (group.needsYou > 0) {
    return (
      <StatusDot
        tone="accent"
        size="sm"
        label={`${group.needsYou} need${group.needsYou === 1 ? "s" : ""} you`}
        data-slot="session-group-needs-you"
      />
    );
  }
  if (group.working > 0) {
    return (
      <StatusDot
        tone="success"
        size="sm"
        label={workspaceWorkingLabel(group.working)}
        data-slot="session-group-working"
      />
    );
  }
  return null;
}

function GroupIdentity({ group }: { group: SessionListGroupSpec }) {
  if (group.badge) return <WorkspaceBadge identity={group.badge} variant="chrome" />;
  if (group.owner) return <ProfileOwnerTag owner={group.owner} />;
  return null;
}

/**
 * One group's body: its own catalog page, paged independently. A group whose
 * query is one (workspace, profile) pair also carries that pair's folders and
 * manual order (BR-11); any other group renders plain rows.
 */
function SessionListGroupBody({
  group,
  ...props
}: GroupRowProps & { group: SessionListGroupSpec }) {
  const { view } = props;
  const catalog = useSessionCatalog(group.scope, sessionListGroupFilters(view, group), true, {
    facets: false,
    lens: group.lens,
  });
  const folders = useSessionFolders({
    workspaceId: group.pair?.workspaceId ?? null,
    profile: group.pair?.profile ?? "",
    enabled: group.pair !== null,
  });
  const threads = buildSessionListThreads(catalog.sessions, "");
  const layout = useSessionListFolderLayout(
    {
      layout: group.pair ? folders.layout : undefined,
      sort: view.sort,
      archived: view.archived,
      worktreeId: view.worktreeId,
      catalog,
    },
    threads,
    props.filtering
  );
  const select = (session: SessionPayload) => {
    if (!catalog.paging) props.onSelectSession(session);
  };
  const empty = emptyForScope(view.archived ? "archived sessions" : "sessions", view.scopeLabel);
  // The grouped dimension is the header: a profile group repeats no owner tag.
  const ownerOf = group.kind === "project" && view.aggregate ? view.ownerOf : undefined;

  if (catalog.failed && catalog.sessions.length === 0) {
    // One unreachable group never blanks the list: it states what happened in
    // its own place and offers the one useful action.
    return (
      <GroupLoadFailure
        retryTestId={`${props.testIdPrefix}-group-${group.key}-retry`}
        onRetry={catalog.retry}
      />
    );
  }
  if (catalog.loading) return <GroupNote>Loading sessions…</GroupNote>;
  return (
    <div className="flex flex-col gap-0.5 pl-2.5">
      {layout.layout && layout.sections ? (
        <>
          <SessionListSections
            ownerOf={ownerOf}
            sections={layout.sections}
            layout={layout.layout}
            dnd={layout.dnd}
            orderable={layout.orderable}
            filtering={props.filtering}
            collapsedThreads={props.collapsedThreads}
            collapsedFolders={layout.collapsedFolders}
            currentSessionId={props.currentSessionId}
            onToggleThread={props.onToggleThread}
            onToggleFolder={layout.toggleFolder}
            onSelectSession={select}
            onNewSessionInFolder={props.onNewSessionInFolder}
            onFolderDialog={layout.setDialog}
            sessionActions={props.sessionActions}
            testIdPrefix={props.testIdPrefix}
            emptyMessage={`${empty}.`}
          />
          <SessionFolderDialogs
            state={layout.dialog}
            onClose={() => layout.setDialog(null)}
            onCreate={layout.layout.createFolder}
            onRename={layout.layout.renameFolder}
            onDelete={layout.layout.deleteFolder}
          />
        </>
      ) : (
        <GroupThreads
          {...props}
          threads={threads}
          emptyMessage={`${empty}.`}
          ownerOf={ownerOf}
          onSelectSession={select}
        />
      )}
      <GroupPager catalog={catalog} />
    </div>
  );
}

/**
 * Group-by groups (ADR-004): membership, counts, and the collapsed needs-you or
 * working dot come from server facets, so an off-page group is never missing.
 * Every expanded group reads and pages its own query; a failed group retries in
 * place. Inside a project group rows omit the project badge; inside a profile
 * group they omit the profile tag.
 */
export function SessionListGroups(props: GroupRowProps) {
  const { view, testIdPrefix } = props;
  if (view.groupsError) {
    return (
      <GroupLoadFailure
        retryTestId={`${testIdPrefix}-groups-retry`}
        onRetry={view.groupsError.retry}
      />
    );
  }
  if (view.groupsLoading) return <GroupNote>Loading sessions…</GroupNote>;
  if (view.groups.length === 0) {
    return (
      <p className="px-3 py-8 text-center text-small-body text-muted">
        {emptyForScope(view.archived ? "archived sessions" : "sessions", view.scopeLabel)}.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-1">
      {view.groups.map(group => {
        const collapsed = view.collapsedGroupKeys.has(group.key);
        return (
          <section key={group.key} data-collapsed={collapsed ? "true" : undefined}>
            <button
              type="button"
              aria-expanded={!collapsed}
              data-testid={`${testIdPrefix}-group-${group.key}`}
              className="flex min-h-7 w-full min-w-0 items-center gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={() => view.toggleGroup(group.key)}
            >
              <Icon
                as={ChevronRight}
                size="sm"
                className={cn(
                  "shrink-0 text-faint transition-transform",
                  !collapsed && "rotate-90"
                )}
              />
              <span className="flex min-w-0 flex-1 items-center">
                <GroupIdentity group={group} />
              </span>
              {collapsed ? <GroupSignal group={group} /> : null}
              <span className="font-mono text-micro text-faint">{group.total}</span>
            </button>
            {collapsed ? null : (
              <SessionListBadgeContext value={group.kind === "project" ? undefined : view.badgeOf}>
                <SessionListGroupBody {...props} group={group} />
              </SessionListBadgeContext>
            )}
          </section>
        );
      })}
    </div>
  );
}

function GroupLoadFailure({ retryTestId, onRetry }: { retryTestId: string; onRetry: () => void }) {
  return (
    <GroupNote>
      <Icon as={TriangleAlert} size="sm" className="text-warning" />
      Couldn&rsquo;t load sessions
      <Button
        variant="ghost"
        size="sm"
        className="ml-auto"
        data-testid={retryTestId}
        onClick={onRetry}
      >
        Retry
      </Button>
    </GroupNote>
  );
}

type GroupCatalog = ReturnType<typeof useSessionCatalog>;

function GroupThreads({
  threads,
  emptyMessage,
  ownerOf,
  onSelectSession,
  ...props
}: Omit<GroupRowProps, "onSelectSession"> & {
  threads: ReturnType<typeof buildSessionListThreads>;
  emptyMessage: string;
  ownerOf: SessionListViewModel["ownerOf"] | undefined;
  onSelectSession: (session: SessionPayload) => void;
}) {
  if (threads.length === 0) return <GroupNote>{emptyMessage}</GroupNote>;
  return threads.map(thread => (
    <SessionListThread
      key={thread.session.id}
      session={thread.session}
      childSessions={thread.childSessions}
      currentSessionId={props.currentSessionId}
      owner={ownerOf?.(thread.session)}
      ownerOf={ownerOf}
      collapsed={props.collapsedThreads.has(thread.session.id)}
      onToggleThread={props.onToggleThread}
      onSelectSession={onSelectSession}
      sessionActions={props.sessionActions}
      testIdPrefix={props.testIdPrefix}
    />
  ));
}

/** A failed refresh over loaded rows, and the group's own previous/next paging. */
function GroupPager({ catalog }: { catalog: GroupCatalog }) {
  return (
    <>
      {catalog.failed && catalog.sessions.length > 0 ? (
        <GroupNote>
          Couldn’t refresh sessions{" "}
          <Button variant="ghost" size="sm" onClick={catalog.retry}>
            Retry
          </Button>
        </GroupNote>
      ) : null}
      {catalog.paging || catalog.previous || catalog.next ? (
        <GroupNote>
          {catalog.paging ? <span role="status">Loading sessions…</span> : null}
          {catalog.previous ? (
            <Button
              variant="ghost"
              size="sm"
              disabled={catalog.paging}
              onClick={catalog.previousPage}
            >
              Previous sessions
            </Button>
          ) : null}
          {catalog.next ? (
            <Button variant="ghost" size="sm" disabled={catalog.paging} onClick={catalog.nextPage}>
              Next sessions
            </Button>
          ) : null}
        </GroupNote>
      ) : null}
    </>
  );
}
