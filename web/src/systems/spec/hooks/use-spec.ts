import { useQuery, type QueryKey } from "@tanstack/react-query";

import { getWorkflowSpec } from "../adapters/spec-api";
import { specKeys } from "../lib/query-keys";
import type { WorkflowSpecDocument } from "../types";

export function useWorkflowSpec(workspaceId: string | null, slug: string | null) {
  return useQuery<WorkflowSpecDocument>({
    queryKey: specKeys.workflow(workspaceId ?? "none", slug ?? "none") as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load workflow spec");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load workflow spec");
      }
      return getWorkflowSpec({ workspaceId, slug });
    },
    enabled: Boolean(workspaceId) && Boolean(slug),
  });
}
