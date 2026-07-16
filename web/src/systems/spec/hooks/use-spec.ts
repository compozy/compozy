import { useQuery, type QueryKey } from "@tanstack/react-query";

import { getWorkflowSpec } from "../adapters/spec-api";
import { specKeys } from "../lib/query-keys";
import type { WorkflowSpecDocument } from "../types";

export function useWorkflowSpec(
  workspaceId: string | null,
  slug: string | null,
  packageId?: string
) {
  return useQuery<WorkflowSpecDocument>({
    queryKey: specKeys.workflow(workspaceId ?? "none", slug ?? "none", packageId) as QueryKey,
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load workflow spec");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load workflow spec");
      }
      return getWorkflowSpec({ workspaceId, slug, packageId });
    },
    enabled: Boolean(workspaceId) && Boolean(slug),
  });
}
