import type { FilesRootRef } from "../adapters/files-api";

function rootKey(root: FilesRootRef): readonly string[] {
  return root.kind === "session"
    ? ["session", root.workspaceId, root.sessionId]
    : ["workspace", root.workspaceId, root.worktreeId ?? ""];
}

export const filesKeys = {
  all: ["files"] as const,
  openTargets: () => [...filesKeys.all, "desktop-open-targets"] as const,
  root: (root: FilesRootRef) => [...filesKeys.all, ...rootKey(root)] as const,
  entriesRoot: (root: FilesRootRef) => [...filesKeys.root(root), "entries"] as const,
  entries: (root: FilesRootRef, path: string, showIgnored: boolean) =>
    [...filesKeys.entriesRoot(root), showIgnored ? "all" : "visible", path] as const,
  contentRoot: (root: FilesRootRef) => [...filesKeys.root(root), "content"] as const,
  content: (root: FilesRootRef, path: string, reveal: boolean) =>
    [...filesKeys.contentRoot(root), path, reveal ? "revealed" : "gated"] as const,
};
