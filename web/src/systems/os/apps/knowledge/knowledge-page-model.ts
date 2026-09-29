import {
  type KnowledgeAgentTier,
  type KnowledgeMemoryItem,
  knowledgeMemoryKey,
  type KnowledgeScope,
  type KnowledgeSelector,
  type MemoryHeader,
  type MemoryType,
} from "@/systems/knowledge";
import type { CreateDestination } from "@/systems/workspace";

interface DecorateOptions {
  scope: KnowledgeScope;
  agentTier?: KnowledgeAgentTier;
  agentName?: string;
  workspaceId?: string;
}

function withLocatorDefaults(memory: MemoryHeader, defaults: DecorateOptions): KnowledgeMemoryItem {
  return {
    ...memory,
    scope: memory.scope ?? defaults.scope,
    agent_tier: memory.agent_tier ?? defaults.agentTier,
    agent_name: memory.agent_name ?? defaults.agentName,
    workspace_id: memory.workspace_id ?? defaults.workspaceId,
  };
}

/** Catalog rows keep a server-provided key; otherwise the key derives from the locator. */
function decorateKnowledgeMemories(
  memories: KnowledgeMemoryItem[] | undefined,
  defaults: DecorateOptions
): KnowledgeMemoryItem[] {
  return (memories ?? []).map(memory => {
    const decorated = withLocatorDefaults(memory, defaults);
    decorated.key = memory.key ?? knowledgeMemoryKey(decorated);
    return decorated;
  });
}

/** Search hits always re-derive their key from the decorated locator. */
function decorateKnowledgeSearchHits(
  hits: readonly { memory: MemoryHeader }[],
  defaults: DecorateOptions
): KnowledgeMemoryItem[] {
  return hits.map(hit => {
    const decorated = withLocatorDefaults(hit.memory, defaults);
    decorated.key = knowledgeMemoryKey(decorated);
    return decorated;
  });
}

function selectorFromMemory(memory: KnowledgeMemoryItem, profile: string): KnowledgeSelector {
  return {
    profile,
    scope: memory.scope,
    workspaceId: memory.workspace_id,
    agentName: memory.agent_name,
    agentTier: memory.agent_tier,
  };
}

function describeError(error: unknown, fallback: string): string | null {
  if (!error) {
    return null;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

interface ListSelectorInput {
  profile: string;
  activeScope: KnowledgeScope;
  listWorkspaceId: string | null | undefined;
  activeWorkspaceId: string | null | undefined;
  agentName: string;
  agentTier: KnowledgeAgentTier;
}

function agentSelector(input: ListSelectorInput): KnowledgeSelector | null {
  if (!input.agentName) {
    return null;
  }
  return {
    profile: input.profile,
    scope: "agent",
    agentName: input.agentName,
    agentTier: input.agentTier,
    workspaceId:
      input.agentTier === "workspace" ? (input.activeWorkspaceId ?? undefined) : undefined,
  };
}

/** The selector the list reads; null when the scope still needs a project or an agent. */
function buildListSelector(input: ListSelectorInput): KnowledgeSelector | null {
  if (input.activeScope === "workspace") {
    return input.listWorkspaceId
      ? { profile: input.profile, scope: "workspace", workspaceId: input.listWorkspaceId }
      : null;
  }
  if (input.activeScope === "agent") {
    return agentSelector(input);
  }
  return { profile: input.profile, scope: "profile" };
}

/** New entries land where the menubar points, except agent memory which follows the list. */
function buildCreateSelector(
  activeScope: KnowledgeScope,
  listSelector: KnowledgeSelector | null,
  destination: Pick<CreateDestination, "scope" | "workspaceId">,
  profile: string
): KnowledgeSelector | null {
  if (activeScope === "agent") {
    return listSelector;
  }
  if (destination.scope === "workspace" && destination.workspaceId) {
    return { profile, scope: "workspace", workspaceId: destination.workspaceId };
  }
  return { profile, scope: "profile" };
}

function createDefaultType(createSelector: KnowledgeSelector | null): MemoryType {
  return createSelector?.scope === "workspace" ? "project" : "user";
}

function resolveProjectName(
  workspaceId: string | undefined,
  knownWorkspaces: readonly { id: string; name: string }[] | undefined,
  activeWorkspaceId: string | null | undefined
): string | undefined {
  if (!workspaceId) {
    return undefined;
  }
  const known = knownWorkspaces?.find(workspace => workspace.id === workspaceId);
  if (known) {
    return known.name;
  }
  return workspaceId === activeWorkspaceId ? "This project" : undefined;
}

export {
  buildCreateSelector,
  buildListSelector,
  createDefaultType,
  decorateKnowledgeMemories,
  decorateKnowledgeSearchHits,
  describeError,
  resolveProjectName,
  selectorFromMemory,
};
export type { DecorateOptions };
