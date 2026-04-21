import { useQuery, type QueryKey } from "@tanstack/react-query";

import { getWorkflowMemoryFile, getWorkflowMemoryIndex } from "../adapters/memory-api";
import { memoryKeys } from "../lib/query-keys";
import type { MarkdownDocument, WorkflowMemoryIndex } from "../types";

export function useWorkflowMemoryIndex(workspaceId: string | null, slug: string | null) {
  return useQuery<WorkflowMemoryIndex>({
    queryKey: memoryKeys.index(workspaceId ?? "none", slug ?? "none") as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load memory index");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load memory index");
      }
      return getWorkflowMemoryIndex({ workspaceId, slug });
    },
    enabled: Boolean(workspaceId) && Boolean(slug),
  });
}

export function useWorkflowMemoryFile(
  workspaceId: string | null,
  slug: string | null,
  fileId: string | null
) {
  return useQuery<MarkdownDocument>({
    queryKey: memoryKeys.file(workspaceId ?? "none", slug ?? "none", fileId ?? "none") as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load a memory file");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load a memory file");
      }
      if (!fileId) {
        throw new Error("file id is required to load a memory file");
      }
      return getWorkflowMemoryFile({ workspaceId, slug, fileId });
    },
    enabled: Boolean(workspaceId) && Boolean(slug) && Boolean(fileId),
  });
}
