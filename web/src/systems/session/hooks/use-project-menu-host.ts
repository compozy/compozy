import { useState } from "react";

import { useOpenProjectFiles } from "@/systems/os";
import { useProfileReadScope } from "@/systems/profiles";
import type { ProjectMenuHost, ProjectMenuMove, WorkspacePayload } from "@/systems/workspace";

import { useSessionCreateActions } from "./use-session-create";
import { useSessionListGroupFolderCreate } from "./use-session-list-group-folder-create";
import { useWorkspaceLayout, type WorkspaceLayout } from "./use-workspace-layout";

/** The shell's project-menu verbs and the New folder dialog state they open. */
export function useProjectMenuHost() {
  const { openForAgent } = useSessionCreateActions();
  const openFiles = useOpenProjectFiles();
  const { params } = useProfileReadScope();
  const profile = "profile" in params ? params.profile : null;
  const [layoutWanted, setLayoutWanted] = useState(false);
  const { layout } = useWorkspaceLayout({
    profile: profile ?? "",
    enabled: layoutWanted && profile !== null,
  });
  const [folderProject, setFolderProject] = useState<WorkspacePayload | null>(null);
  const folder = useSessionListGroupFolderCreate({
    workspaceId: folderProject?.id ?? null,
    profile,
  });
  const host: ProjectMenuHost = {
    menuOpened: () => setLayoutWanted(true),
    verbsFor: workspace => ({
      newSession: () =>
        openForAgent("", { workspaceId: workspace.id, ...(profile ? { profile } : {}) }),
      newFolder: () => {
        setFolderProject(workspace);
        folder.openDialog();
      },
      openFiles: openFiles ? () => openFiles(workspace.id) : undefined,
      move: layout && profile !== null ? projectFolderMove(layout, workspace.id) : undefined,
    }),
  };
  return { host, folder, folderProject };
}

/** Move ▸ without manual order: these surfaces do not show the folders' project order. */
function projectFolderMove(layout: WorkspaceLayout, workspaceId: string): ProjectMenuMove {
  const placement = layout.placements.find(row => row.workspace_id === workspaceId);
  return {
    folders: [...layout.folders].sort((left, right) => left.position - right.position),
    folderId: placement?.folder_id ?? null,
    onMove: folderId => layout.moveWorkspace(workspaceId, folderId),
  };
}
