import { useState } from "react";

import { useArchivedWorkspaces, useWorkspaces, useWorktrees } from "@/systems/workspace";

import {
  filesAppProjectStatus,
  filesAppRootOptions,
  filesSwitcherProjects,
} from "../lib/files-app-roots";
import { fileViewerTarget, type FileViewerTarget } from "../lib/file-viewer-target";
import { useFilesAppPlace } from "./use-files-app-place";

/** The Files window's view model: its project's catalog standing, roots, and open file. */
export function useFilesAppWindow(workspaceId: string) {
  const workspaces = useWorkspaces();
  const archived = useArchivedWorkspaces();
  const worktrees = useWorktrees(workspaceId);
  const place = useFilesAppPlace(workspaceId);
  const [target, setTarget] = useState<FileViewerTarget | null>(() =>
    place.path ? fileViewerTarget(place.path) : null
  );
  const workspace = [...(workspaces.data ?? []), ...(archived.data ?? [])].find(
    candidate => candidate.id === workspaceId
  );
  return {
    status: filesAppProjectStatus(workspaceId, {
      active: workspaces.data,
      archived: archived.data,
    }),
    projects: filesSwitcherProjects(workspaces.data),
    workspaceName: workspace?.name ?? workspaceId,
    rootOptions: filesAppRootOptions(worktrees.data?.worktrees),
    place,
    target,
    openFile: (path: string) => {
      place.openPath(path);
      setTarget(fileViewerTarget(path));
    },
  };
}
