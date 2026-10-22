import { useQueries, useQuery } from "@tanstack/react-query";

import { agentsListOptions, type AgentPayload } from "@/systems/agent";
import { extensionsListOptions, type ExtensionEntry } from "@/systems/extensions";
import {
  loopCatalogBatchEntries,
  loopsAggregateOptions,
  type LoopCatalogEntry,
} from "@/systems/loops";
import type { ProfileOwnerLabel } from "@/systems/profiles";
import { worktreesListOptions } from "@/systems/workspace";

import type { QueryState } from "../lib/os-palette-domain-search";

export interface PaletteWorkspaceLoop {
  readonly loop: LoopCatalogEntry;
  readonly workspaceId: string;
}

export interface PaletteWorkspaceAgent {
  readonly agent: AgentPayload;
  readonly workspaceId: string;
}

export interface PaletteWorkspaceExtension {
  readonly extension: ExtensionEntry;
  readonly workspaceId: string;
}

export interface PaletteWorkspaceWorktree extends ProfileOwnerLabel {
  readonly id: string;
  readonly name: string;
  readonly branch: string;
  readonly state: string;
  readonly workspace_id: string;
}

export interface OsPaletteWorkspaceCatalogs {
  readonly loops: readonly PaletteWorkspaceLoop[];
  readonly loopTotal: number;
  readonly loopState: QueryState;
  readonly workspaceAgents: readonly PaletteWorkspaceAgent[];
  readonly workspaceAgentState: QueryState;
  readonly publishedExtensions: readonly ExtensionEntry[];
  readonly workspaceExtensions: readonly PaletteWorkspaceExtension[];
  readonly workspaceExtensionState: QueryState;
  readonly worktrees: readonly PaletteWorkspaceWorktree[];
  readonly worktreeTotal: number;
  readonly worktreeState: QueryState;
}

export interface UseOsPaletteWorkspaceCatalogsOptions {
  readonly profile: string;
  readonly workspaceIds: readonly string[];
  readonly loopsEnabled: boolean;
  readonly agentsEnabled: boolean;
  readonly extensionsEnabled: boolean;
  readonly worktreesEnabled: boolean;
}

function queryState(
  results: ReadonlyArray<{ isLoading: boolean; isError: boolean; error: unknown }>
): QueryState {
  return {
    isLoading: results.some(result => result.isLoading),
    isError: results.some(result => result.isError),
    error: results.find(result => result.error != null)?.error ?? null,
  };
}

/**
 * Globe-mode catalogs. Loops read the All projects definitions batch (User
 * rows plus every project's rows) in one query; catalogs that have no
 * all-workspaces list API yet stay one query per workspace.
 */
export function useOsPaletteWorkspaceCatalogs({
  profile,
  workspaceIds,
  loopsEnabled,
  agentsEnabled,
  extensionsEnabled,
  worktreesEnabled,
}: UseOsPaletteWorkspaceCatalogsOptions): OsPaletteWorkspaceCatalogs {
  const ids = workspaceIds.filter(id => id.trim() !== "");
  const loopQuery = useQuery(
    loopsAggregateOptions({ all_workspaces: true }, { profile }, loopsEnabled)
  );
  const agentQueries = useQueries({
    queries: ids.map(workspaceId => ({
      ...agentsListOptions(workspaceId),
      enabled: agentsEnabled,
    })),
  });
  const extensionQueries = useQueries({
    queries: ids.map(workspaceId => extensionsListOptions({ workspaceId }, extensionsEnabled)),
  });
  const publishedExtensions = useQuery(extensionsListOptions({}, extensionsEnabled));
  const worktreeQueries = useQueries({
    queries: ids.map(workspaceId => worktreesListOptions(workspaceId, worktreesEnabled)),
  });

  // A User row has no `workspace_id`: it opens bound to User.
  const loops: PaletteWorkspaceLoop[] = loopCatalogBatchEntries(loopQuery.data?.entries ?? []).map(
    loop => ({ loop, workspaceId: loop.workspace_id ?? "" })
  );

  const workspaceAgents: PaletteWorkspaceAgent[] = [];
  ids.forEach((workspaceId, index) => {
    for (const agent of agentQueries[index]?.data ?? []) {
      workspaceAgents.push({ agent, workspaceId: agent.workspace_id ?? workspaceId });
    }
  });

  const workspaceExtensions: PaletteWorkspaceExtension[] = [];
  ids.forEach((workspaceId, index) => {
    for (const extension of extensionQueries[index]?.data ?? []) {
      workspaceExtensions.push({ extension, workspaceId });
    }
  });

  const worktrees: PaletteWorkspaceWorktree[] = [];
  ids.forEach((workspaceId, index) => {
    for (const worktree of worktreeQueries[index]?.data?.worktrees ?? []) {
      worktrees.push({
        id: worktree.id,
        name: worktree.name,
        branch: worktree.branch,
        state: worktree.state,
        workspace_id: worktree.workspace_id ?? workspaceId,
        profile_id: worktree.profile_id,
        profile_name: worktree.profile_name,
        profile_color: worktree.profile_color,
        profile_icon: worktree.profile_icon,
        profile_emoji: worktree.profile_emoji,
        profile_archived: worktree.profile_archived,
      });
    }
  });

  return {
    loops,
    loopTotal: loops.length,
    loopState: queryState([loopQuery]),
    workspaceAgents,
    workspaceAgentState: queryState(agentQueries),
    publishedExtensions: publishedExtensions.data ?? [],
    workspaceExtensions,
    workspaceExtensionState: queryState([publishedExtensions, ...extensionQueries]),
    worktrees,
    worktreeTotal: worktrees.length,
    worktreeState: queryState(worktreeQueries),
  };
}
