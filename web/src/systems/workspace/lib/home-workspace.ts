export interface WorkspaceHomeCandidate {
  id: string;
  root_dir?: string | null;
}

export interface HomeWorkspacePartition<T extends WorkspaceHomeCandidate> {
  homeWorkspace: T | undefined;
  projectWorkspaces: T[];
}

export function isHomeWorkspace<T extends WorkspaceHomeCandidate>(
  workspace: T,
  userHomeDir: string | undefined
): boolean {
  return Boolean(userHomeDir && userHomeDir.length > 0 && workspace.root_dir === userHomeDir);
}

export function splitHomeWorkspace<T extends WorkspaceHomeCandidate>(
  workspaces: ReadonlyArray<T> | undefined,
  userHomeDir: string | undefined
): HomeWorkspacePartition<T> {
  const workspaceList = workspaces ?? [];
  const homeWorkspace = workspaceList.find(workspace => isHomeWorkspace(workspace, userHomeDir));
  if (!homeWorkspace) {
    return { homeWorkspace: undefined, projectWorkspaces: [...workspaceList] };
  }

  return {
    homeWorkspace,
    projectWorkspaces: workspaceList.filter(workspace => workspace.id !== homeWorkspace.id),
  };
}
