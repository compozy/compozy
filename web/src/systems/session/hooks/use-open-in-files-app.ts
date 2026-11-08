import { useQueryClient } from "@tanstack/react-query";

import { requestFilesAppPlace, type FileViewerTarget } from "@/systems/files";
import { useOpenProjectFiles } from "@/systems/os";

import { readFilePathResolutionBase, type SessionScope } from "../lib/file-path-resolution-base";

export type OpenInFilesApp = (target: FileViewerTarget) => void;

/**
 * Opens a session file in its project's Files window, on the session's root (its bound worktree,
 * else the checkout) at the target's line. A new window opens as a tab of the focused window
 * (the session's); the project's existing Files window is retargeted instead. `null` outside the
 * desktop shell, where there is no window to open.
 */
export function useOpenInFilesApp(scope: SessionScope): OpenInFilesApp | null {
  const queryClient = useQueryClient();
  const openFiles = useOpenProjectFiles();
  if (openFiles === null) return null;
  return target => {
    void readFilePathResolutionBase(queryClient, scope).then(({ worktreeId }) => {
      requestFilesAppPlace(
        scope.workspaceId,
        { worktreeId, path: target.kind === "file" ? target.path : null },
        target
      );
      openFiles(scope.workspaceId);
    });
  };
}
