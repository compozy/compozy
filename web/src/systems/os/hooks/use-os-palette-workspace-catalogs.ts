import { queryOptions, useQueries, useQuery } from "@tanstack/react-query";

import { agentsListOptions, type AgentPayload } from "@/systems/agent";
import { extensionsListOptions, type ExtensionEntry } from "@/systems/extensions";
import { listMemories, memoriesListOptions, type MemoryHeader } from "@/systems/knowledge";
import { listLoops, loopsCatalogOptions, type LoopCatalogEntry } from "@/systems/loops";
import { networkChannelsOptions, type NetworkChannelSummary } from "@/systems/network";
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

export interface PaletteWorkspaceChannel extends NetworkChannelSummary {
  readonly workspace_id: string;
}

export interface PaletteWorkspaceWorktree {
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
  readonly channels: readonly PaletteWorkspaceChannel[];
  readonly channelTotal: number;
  readonly channelState: QueryState;
  readonly workspaceMemories: readonly MemoryHeader[];
  readonly workspaceMemoryTotal: number;
  readonly workspaceMemoryState: QueryState;
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
  readonly networkEnabled: boolean;
  readonly knowledgeEnabled: boolean;
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

async function fetchAllWorkspaceLoops(workspaceId: string, signal: AbortSignal) {
  const loops: LoopCatalogEntry[] = [];
  const seenCursors = new Set<string>();
  let cursor: string | undefined;
  let total = 0;
  for (;;) {
    const page = await listLoops(workspaceId, { cursor }, signal);
    loops.push(...page.loops);
    total = page.page.total;
    if (!page.page.has_more) return { loops, total };
    const nextCursor = page.page.next_cursor?.trim();
    if (!nextCursor || seenCursors.has(nextCursor)) {
      throw new Error(`Loop catalog returned an invalid continuation for ${workspaceId}`);
    }
    seenCursors.add(nextCursor);
    cursor = nextCursor;
  }
}

async function fetchAllWorkspaceMemories(
  profile: string,
  workspaceId: string,
  signal: AbortSignal
) {
  const memories: MemoryHeader[] = [];
  const seenCursors = new Set<string>();
  let cursor: string | undefined;
  let total = 0;
  for (;;) {
    const page = await listMemories(
      { profile, scope: "workspace", workspaceId, includeSystem: false, cursor },
      signal
    );
    memories.push(...page.memories);
    total = page.page.total;
    if (!page.page.has_more) return { memories, total };
    const nextCursor = page.page.next_cursor?.trim();
    if (!nextCursor || seenCursors.has(nextCursor)) {
      throw new Error(`Memory catalog returned an invalid continuation for ${workspaceId}`);
    }
    seenCursors.add(nextCursor);
    cursor = nextCursor;
  }
}

/**
 * Globe-mode catalogs that have no all-workspaces list API. Each workspace is
 * its own query under the domain option factory.
 */
export function useOsPaletteWorkspaceCatalogs({
  profile,
  workspaceIds,
  loopsEnabled,
  networkEnabled,
  knowledgeEnabled,
  agentsEnabled,
  extensionsEnabled,
  worktreesEnabled,
}: UseOsPaletteWorkspaceCatalogsOptions): OsPaletteWorkspaceCatalogs {
  const ids = workspaceIds.filter(id => id.trim() !== "");
  const loopQueries = useQueries({
    queries: ids.map(workspaceId => {
      const base = loopsCatalogOptions(workspaceId, {});
      const queryKey: readonly unknown[] = [...base.queryKey, "palette-all-pages"];
      return queryOptions({
        queryKey,
        queryFn: ({ signal }) => fetchAllWorkspaceLoops(workspaceId, signal),
        enabled: loopsEnabled,
        staleTime: 15_000,
      });
    }),
  });
  const channelQueries = useQueries({
    queries: ids.map(workspaceId => networkChannelsOptions(workspaceId, {}, networkEnabled)),
  });
  const memoryQueries = useQueries({
    queries: ids.map(workspaceId => {
      const base = memoriesListOptions({
        profile,
        scope: "workspace",
        workspaceId,
        includeSystem: false,
      });
      const queryKey: readonly unknown[] = [...base.queryKey, "palette-all-pages"];
      return queryOptions({
        queryKey,
        queryFn: ({ signal }) => fetchAllWorkspaceMemories(profile, workspaceId, signal),
        enabled: knowledgeEnabled,
        staleTime: 30_000,
      });
    }),
  });
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

  const loops: PaletteWorkspaceLoop[] = [];
  let loopTotal = 0;
  ids.forEach((workspaceId, index) => {
    const catalog = loopQueries[index]?.data;
    loopTotal += catalog?.total ?? 0;
    for (const loop of catalog?.loops ?? []) {
      loops.push({ loop, workspaceId });
    }
  });

  const channels: PaletteWorkspaceChannel[] = [];
  ids.forEach((workspaceId, index) => {
    for (const channel of channelQueries[index]?.data?.channels ?? []) {
      channels.push({ ...channel, workspace_id: channel.workspace_id ?? workspaceId });
    }
  });

  const workspaceMemories: MemoryHeader[] = [];
  let workspaceMemoryTotal = 0;
  for (const result of memoryQueries) {
    workspaceMemoryTotal += result.data?.total ?? 0;
    workspaceMemories.push(...(result.data?.memories ?? []));
  }

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
      });
    }
  });

  return {
    loops,
    loopTotal,
    loopState: queryState(loopQueries),
    channels,
    channelTotal: channels.length,
    channelState: queryState(channelQueries),
    workspaceMemories,
    workspaceMemoryTotal,
    workspaceMemoryState: queryState(memoryQueries),
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
