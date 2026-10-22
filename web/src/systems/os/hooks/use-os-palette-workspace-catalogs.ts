import { useInfiniteQuery, useQueries, useQuery } from "@tanstack/react-query";

import { agentAggregateOptions, type AgentPayload } from "@/systems/agent";
import { extensionsListOptions, type ExtensionEntry } from "@/systems/extensions";
import {
  loopCatalogBatchEntries,
  loopsAggregateOptions,
  type LoopCatalogEntry,
} from "@/systems/loops";
import type { ProfileOwnerLabel } from "@/systems/profiles";
import { useWorktreeAggregate } from "@/systems/workspace";

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

/** The palette reads one bounded page of the All projects agent catalog. */
const PALETTE_AGENT_LIMIT = 100;

/**
 * Globe-mode catalogs. Loops, agents and worktrees read their All projects
 * aggregates (User rows plus every project's) in one query each; extensions,
 * which have no all-workspaces list API, stay one query per workspace.
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
  // User agents plus every project's, one row per (project, name).
  const agentAggregate = useInfiniteQuery({
    ...agentAggregateOptions([], { profile, limit: PALETTE_AGENT_LIMIT }),
    enabled: agentsEnabled,
  });
  const extensionQueries = useQueries({
    queries: ids.map(workspaceId => extensionsListOptions({ workspaceId }, extensionsEnabled)),
  });
  const publishedExtensions = useQuery(extensionsListOptions({}, extensionsEnabled));
  const worktreeAggregate = useWorktreeAggregate([], { enabled: worktreesEnabled });

  // A User row has no `workspace_id`: it opens bound to User.
  const loops: PaletteWorkspaceLoop[] = loopCatalogBatchEntries(loopQuery.data?.entries ?? []).map(
    loop => ({ loop, workspaceId: loop.workspace_id ?? "" })
  );

  const workspaceAgents: PaletteWorkspaceAgent[] = (
    agentAggregate.data?.pages[0]?.agents ?? []
  ).map(item => ({ agent: item.agent, workspaceId: item.agent.workspace_id ?? "" }));

  const workspaceExtensions: PaletteWorkspaceExtension[] = [];
  ids.forEach((workspaceId, index) => {
    for (const extension of extensionQueries[index]?.data ?? []) {
      workspaceExtensions.push({ extension, workspaceId });
    }
  });

  const worktrees: PaletteWorkspaceWorktree[] = (worktreeAggregate.data?.worktrees ?? []).map(
    worktree => ({
      id: worktree.id,
      name: worktree.name,
      branch: worktree.branch,
      state: worktree.state,
      workspace_id: worktree.workspace_id,
      profile_id: worktree.profile_id,
      profile_name: worktree.profile_name,
      profile_color: worktree.profile_color,
      profile_icon: worktree.profile_icon,
      profile_emoji: worktree.profile_emoji,
      profile_archived: worktree.profile_archived,
    })
  );

  return {
    loops,
    loopTotal: loops.length,
    loopState: queryState([loopQuery]),
    workspaceAgents,
    workspaceAgentState: queryState([agentAggregate]),
    publishedExtensions: publishedExtensions.data ?? [],
    workspaceExtensions,
    workspaceExtensionState: queryState([publishedExtensions, ...extensionQueries]),
    worktrees,
    worktreeTotal: worktrees.length,
    worktreeState: queryState([worktreeAggregate]),
  };
}
