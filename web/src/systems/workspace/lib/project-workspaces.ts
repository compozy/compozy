export interface WorkspaceHomeCandidate {
  id: string;
  root_dir?: string | null;
}

export interface ProjectWorkspacePartition<T extends WorkspaceHomeCandidate> {
  homeWorkspace: T | undefined;
  projectWorkspaces: T[];
}

/** Trailing separators vary between daemon paths and `$HOME`; compare without them. */
function normalizeRootDir(path: string): string {
  let end = path.length;
  while (end > 1 && path.charCodeAt(end - 1) === 47) {
    end -= 1;
  }
  return path.slice(0, end);
}

/** True when this registration is the daemon's operator-home row (`$HOME`). */
export function isOperatorHomeWorkspace<T extends WorkspaceHomeCandidate>(
  workspace: T,
  userHomeDir: string | undefined
): boolean {
  if (!userHomeDir || userHomeDir.length === 0 || !workspace.root_dir) {
    return false;
  }
  return normalizeRootDir(workspace.root_dir) === normalizeRootDir(userHomeDir);
}

/**
 * Split the catalog into the hidden operator-home registration and the project
 * workspaces the UI may list. Home is never a selectable project row.
 */
export function partitionProjectWorkspaces<T extends WorkspaceHomeCandidate>(
  workspaces: ReadonlyArray<T> | undefined,
  userHomeDir: string | undefined
): ProjectWorkspacePartition<T> {
  const workspaceList = workspaces ?? [];
  const homeWorkspace = workspaceList.find(workspace =>
    isOperatorHomeWorkspace(workspace, userHomeDir)
  );
  if (!homeWorkspace) {
    return { homeWorkspace: undefined, projectWorkspaces: [...workspaceList] };
  }
  return {
    homeWorkspace,
    projectWorkspaces: workspaceList.filter(workspace => workspace.id !== homeWorkspace.id),
  };
}
