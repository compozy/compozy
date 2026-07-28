import { isHomeWorkspace } from "@/systems/workspace";

interface ActiveWorkspaceScopeCandidate {
  id: string;
  root_dir?: string | null;
}

export type ActiveTaskScopeFilter =
  | { scope: "global"; workspace?: undefined }
  | { scope: "workspace"; workspace: string };

export function taskScopeForActiveWorkspace(
  activeWorkspace: ActiveWorkspaceScopeCandidate | null | undefined,
  userHomeDir: string | undefined
): ActiveTaskScopeFilter | null {
  if (!activeWorkspace || !userHomeDir) {
    return null;
  }

  if (isHomeWorkspace(activeWorkspace, userHomeDir)) {
    return { scope: "global" };
  }

  return { scope: "workspace", workspace: activeWorkspace.id };
}
