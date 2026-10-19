import { Alert, AlertActions, AlertDescription, Button } from "@compozy/ui";
import { ProfileOwnerTag } from "@/systems/profiles";

import { useSessionFolders } from "../../hooks/use-session-folders";
import { useSessionListFolderLayout } from "../../hooks/use-session-list-folder-layout";
import type {
  SessionListViewModel,
  SessionProfileFolderGroup,
} from "../../hooks/use-session-list-view";
import type { SessionThreadModel } from "../../lib/session-list-threads";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import { SessionListSections, type SessionListSectionsProps } from "./session-list-sections";
import { SessionListThreadList } from "./session-list-thread-list";

type ProfileGroupProps = Pick<
  SessionListSectionsProps,
  | "filtering"
  | "collapsedThreads"
  | "currentSessionId"
  | "onToggleThread"
  | "onSelectSession"
  | "onNewSessionInFolder"
  | "selection"
  | "sessionActions"
  | "testIdPrefix"
> & {
  group: SessionProfileFolderGroup;
  view: SessionListViewModel;
  threads: readonly SessionThreadModel[];
};

export function SessionListProfileGroup({ group, view, threads, ...props }: ProfileGroupProps) {
  const state = useSessionFolders({
    workspaceId: group.workspaceId,
    profile: group.owner.name,
    enabled: true,
  });
  const folders = useSessionListFolderLayout(
    { ...view, layout: state.layout },
    threads,
    props.filtering
  );
  const { layout, sections } = folders;
  if (!state.loadError && threads.length === 0 && (!sections || sections.folders.length === 0))
    return null;
  return (
    <section
      aria-label={group.owner.name}
      data-testid={`${props.testIdPrefix}-profile-${group.owner.id}`}
    >
      <div className="px-2 py-2">
        <ProfileOwnerTag owner={group.owner} />
      </div>
      {state.loadError ? (
        <Alert variant="warning">
          <AlertDescription>Couldn&rsquo;t load folders.</AlertDescription>
          <Button size="xs" variant="secondary" onClick={state.loadError.retry}>
            Retry
          </Button>
        </Alert>
      ) : null}
      {layout && sections ? (
        <SessionListSections
          {...props}
          ownerOf={view.ownerOf}
          onNewSessionInFolder={group.canCreateSession ? props.onNewSessionInFolder : undefined}
          sections={sections}
          layout={layout}
          dnd={props.selection.mode ? undefined : folders.dnd}
          orderable={folders.orderable}
          collapsedFolders={folders.collapsedFolders}
          onToggleFolder={folders.toggleFolder}
          onFolderDialog={folders.setDialog}
          emptyMessage="No sessions."
        />
      ) : (
        <SessionListThreadList
          {...props}
          threads={threads}
          ownerOf={view.ownerOf}
          emptyMessage="No sessions."
        />
      )}
      {layout ? (
        <SessionFolderDialogs
          state={folders.dialog}
          onClose={() => folders.setDialog(null)}
          onCreate={layout.createFolder}
          onRename={layout.renameFolder}
          onDelete={layout.deleteFolder}
        />
      ) : null}
    </section>
  );
}

export function SessionListProfileStatus({ view }: { view: SessionListViewModel }) {
  return (
    <>
      {view.profileFoldersLoading ? (
        <p role="status" className="px-3 py-2 text-small-body text-muted">
          Loading profiles…
        </p>
      ) : null}
      {view.profileFoldersError ? (
        <Alert className="mx-3 my-1 w-auto" variant="warning">
          <AlertDescription>Couldn&rsquo;t load profiles.</AlertDescription>
          <AlertActions>
            <Button size="xs" variant="secondary" onClick={view.profileFoldersError.retry}>
              Retry
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
    </>
  );
}

export function SessionListProfileGroups({
  groups,
  view,
  threads,
  emptyMessage,
  footer,
  ...props
}: Omit<ProfileGroupProps, "group"> & {
  groups: readonly SessionProfileFolderGroup[];
  emptyMessage: string;
  footer?: React.ReactNode;
}) {
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
      aria-busy={view.catalog?.paging || undefined}
    >
      {threads.length === 0 ? (
        <p className="px-2 py-3 text-small-body text-muted">{emptyMessage}</p>
      ) : null}
      {groups.map(group => (
        <SessionListProfileGroup
          {...props}
          key={group.owner.id}
          group={group}
          view={view}
          threads={threads.filter(thread => thread.session.profile_id === group.owner.id)}
          onSelectSession={session => {
            if (!view.catalog?.paging) props.onSelectSession(session);
          }}
        />
      ))}
      {footer}
    </div>
  );
}
