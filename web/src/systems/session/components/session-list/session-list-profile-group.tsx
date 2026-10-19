import { Alert, AlertDescription, Button } from "@compozy/ui";
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

export function SessionListProfileGroup({
  group,
  view,
  threads,
  ...props
}: Pick<
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
}) {
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
