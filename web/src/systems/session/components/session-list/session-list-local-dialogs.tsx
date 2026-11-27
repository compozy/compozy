import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type {
  SessionFolderDialogState,
  useSessionListFolderLayout,
} from "../../hooks/use-session-list-folder-layout";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { WorkspaceFolderPayload } from "../../types";
import { SessionFolderDialogs } from "./session-folder-dialogs";

export function SessionListLocalDialogs({
  folders,
  projectLayout,
  projectFolderDialog,
  setProjectFolderDialog,
  scopedRemoteSource,
}: {
  folders: ReturnType<typeof useSessionListFolderLayout>;
  projectLayout: NonNullable<SessionListViewModel["projectFolders"]>["layout"] | undefined;
  projectFolderDialog: SessionFolderDialogState<WorkspaceFolderPayload>;
  setProjectFolderDialog: (state: SessionFolderDialogState<WorkspaceFolderPayload>) => void;
  scopedRemoteSource: ComputerSessionsSource | null;
}) {
  if (scopedRemoteSource) return null;
  return (
    <>
      {folders.layout ? (
        <SessionFolderDialogs
          state={folders.dialog}
          onClose={() => folders.setDialog(null)}
          onCreate={folders.layout.createFolder}
          onRename={folders.layout.renameFolder}
          onDelete={folders.layout.deleteFolder}
        />
      ) : null}
      {projectLayout ? (
        <SessionFolderDialogs
          holds="projects"
          state={projectFolderDialog}
          onClose={() => setProjectFolderDialog(null)}
          onCreate={projectLayout.createFolder}
          onRename={projectLayout.renameFolder}
          onDelete={projectLayout.deleteFolder}
        />
      ) : null}
    </>
  );
}
