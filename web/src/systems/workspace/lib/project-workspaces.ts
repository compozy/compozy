export interface WorkspaceHomeCandidate {
  id: string;
  root_dir?: string | null;
}

/** Trailing separators vary between daemon paths and `$HOME`; compare without them. */
export function normalizeRootDir(path: string): string {
  let end = path.length;
  while (end > 1 && path.charCodeAt(end - 1) === 47) {
    end -= 1;
  }
  return path.slice(0, end);
}

/**
 * True when a folder is the operator home. The daemon refuses to register it
 * (User work runs there without a project), so registration inputs skip it.
 */
export function isOperatorHomeWorkspace<T extends WorkspaceHomeCandidate>(
  workspace: T,
  userHomeDir: string | undefined
): boolean {
  if (!userHomeDir || userHomeDir.length === 0 || !workspace.root_dir) {
    return false;
  }
  return normalizeRootDir(workspace.root_dir) === normalizeRootDir(userHomeDir);
}
