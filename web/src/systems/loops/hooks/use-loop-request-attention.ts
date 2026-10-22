import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import {
  breadthQuery,
  loopBreadthTarget,
  useActiveWorkspace,
  USER_SCOPE_SELECTOR,
  workspaceBadgeIdentity,
  type DataScope,
} from "@/systems/workspace";

import { loopRequestAttentionOptions } from "../lib/query-options";
import type { LoopRequest } from "../types";

export interface LoopRequestAttentionItem {
  request: LoopRequest;
  /** The request's own project; empty for a User-scope run. */
  workspaceId: string;
  /** The project's name, or "User". */
  workspaceLabel: string;
  stale: boolean;
}

export interface LoopRequestAttention {
  pendingCount: number;
  items: LoopRequestAttentionItem[];
  disconnected: boolean;
  loading: boolean;
}

/**
 * Pending Loop requests behind the loops badge: what the Loops listing would
 * show — the breadth (one project, User, or the All projects aggregate) within
 * the profile lens — as one daemon read. The count is the server's aggregate;
 * a failed read counts nothing and reports itself disconnected.
 */
export function useLoopRequestAttention(
  scope: DataScope | null,
  enabled = true,
  poll = true
): LoopRequestAttention {
  const { params } = useProfileReadScope();
  const { registeredWorkspaces } = useActiveWorkspace();
  const names = new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const target = scope === null ? null : loopBreadthTarget(breadthQuery(scope));
  const query = useQuery({
    ...loopRequestAttentionOptions(
      target ?? { kind: "workspace", workspaceId: "" },
      params,
      enabled && target !== null,
      poll ? 15_000 : false
    ),
  });
  const stale = query.isError;
  const fallbackWorkspace = target?.kind === "workspace" ? target.workspaceId : "";
  return {
    pendingCount: stale ? 0 : (query.data?.aggregates.pending ?? 0),
    items: (query.data?.items ?? []).map(request => {
      const workspaceId = request.workspace_id?.trim() || ownWorkspace(fallbackWorkspace);
      return {
        request,
        workspaceId,
        workspaceLabel: workspaceBadgeIdentity(workspaceId, names).name,
        stale,
      };
    }),
    disconnected: stale,
    loading: query.isLoading,
  };
}

/** A workspace route's rows belong to that workspace; `@user` rows have none. */
function ownWorkspace(selector: string): string {
  return selector === USER_SCOPE_SELECTOR ? "" : selector;
}
