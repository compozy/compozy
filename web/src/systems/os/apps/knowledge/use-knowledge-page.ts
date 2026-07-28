import { useState } from "react";

import {
  knowledgeMemoryKey,
  DEFAULT_MEMORY_LIST_LIMIT,
  useDeleteMemory,
  useEditMemory,
  useMemories,
  useMemory,
  useMemoryDecisions,
  useMemorySearch,
  useRevertMemoryDecision,
  useWriteMemory,
  type EditMemoryParams,
  type KnowledgeAgentTier,
  type KnowledgeListFilter,
  type KnowledgeMemoryItem,
  type KnowledgeScope,
  type KnowledgeSelector,
  type MemoryDecision,
  type MemoryEditRequest,
  type MemoryHeader,
  type MemoryType,
  type MemoryWriteRequest,
} from "@/systems/knowledge";
import { useActiveWorkspace } from "@/systems/workspace";

interface DecorateOptions {
  scope: KnowledgeScope;
  agentTier?: KnowledgeAgentTier;
  agentName?: string;
  workspaceId?: string;
}

function decorateKnowledgeMemories(
  memories: MemoryHeader[] | undefined,
  defaults: DecorateOptions
): KnowledgeMemoryItem[] {
  return (memories ?? []).map(memory => {
    const decorated: KnowledgeMemoryItem = {
      ...memory,
      scope: memory.scope ?? defaults.scope,
      agent_tier: memory.agent_tier ?? defaults.agentTier,
      agent_name: memory.agent_name ?? defaults.agentName,
      workspace_id: memory.workspace_id ?? defaults.workspaceId,
    };
    decorated.key = decorated.key ?? knowledgeMemoryKey(decorated);
    return decorated;
  });
}

function selectorFromMemory(memory: KnowledgeMemoryItem): KnowledgeSelector {
  return {
    scope: memory.scope,
    workspaceId: memory.workspace_id,
    agentName: memory.agent_name,
    agentTier: memory.agent_tier,
  };
}

function describeError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

function useKnowledgePage() {
  const { activeWorkspaceId } = useActiveWorkspace();

  const [activeScope, setActiveScope] = useState<KnowledgeScope>("global");
  const [agentName, setAgentName] = useState("");
  const [agentTier, setAgentTier] = useState<KnowledgeAgentTier>("workspace");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedMemoryKey, setSelectedMemoryKey] = useState<string | null>(null);
  const [actionTargetKey, setActionTargetKey] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [revertingDecisionId, setRevertingDecisionId] = useState<string | null>(null);

  const trimmedAgentName = agentName.trim();
  const trimmedSearchQuery = searchQuery.trim();

  let selector: KnowledgeSelector | null = { scope: "global" };
  if (activeScope === "workspace") {
    selector = activeWorkspaceId ? { scope: "workspace", workspaceId: activeWorkspaceId } : null;
  } else if (activeScope === "agent") {
    selector = trimmedAgentName
      ? {
          scope: "agent",
          agentName: trimmedAgentName,
          agentTier,
          workspaceId: agentTier === "workspace" ? (activeWorkspaceId ?? undefined) : undefined,
        }
      : null;
  }

  const decorateOptions: DecorateOptions = {
    scope: activeScope,
    agentTier: activeScope === "agent" ? agentTier : undefined,
    agentName: activeScope === "agent" ? trimmedAgentName : undefined,
    workspaceId: selector?.workspaceId,
  };

  const catalogFilter: KnowledgeListFilter | undefined = selector
    ? {
        ...selector,
        includeSystem: false,
        limit: DEFAULT_MEMORY_LIST_LIMIT,
        sort: "recent",
      }
    : undefined;
  const memoriesQuery = useMemories(catalogFilter, { enabled: Boolean(catalogFilter) });
  const searchEnabled = Boolean(selector) && trimmedSearchQuery.length > 0;
  const searchQueryResult = useMemorySearch(selector ?? undefined, trimmedSearchQuery, {
    enabled: searchEnabled,
  });

  const {
    error: deleteMutationError,
    isPending: isDeletePending,
    mutateAsync: deleteMemoryMutate,
    reset: resetDeleteMutation,
  } = useDeleteMemory();

  const {
    error: editMutationError,
    isPending: isEditPending,
    mutateAsync: editMemoryMutate,
    reset: resetEditMutation,
  } = useEditMemory();

  const {
    error: writeMutationError,
    isPending: isCreatePending,
    mutateAsync: writeMemoryMutate,
    reset: resetWriteMutation,
  } = useWriteMemory();

  const {
    error: revertMutationError,
    isPending: isRevertPending,
    mutateAsync: revertMemoryDecisionMutate,
    reset: resetRevertMutation,
  } = useRevertMemoryDecision();

  const listMemories: KnowledgeMemoryItem[] = searchEnabled
    ? (searchQueryResult.data?.results ?? []).map(result => {
        const decorated: KnowledgeMemoryItem = {
          ...result.memory,
          scope: result.memory.scope ?? activeScope,
          agent_tier: result.memory.agent_tier ?? decorateOptions.agentTier,
          agent_name: result.memory.agent_name ?? decorateOptions.agentName,
          workspace_id: result.memory.workspace_id ?? decorateOptions.workspaceId,
        };
        decorated.key = knowledgeMemoryKey(decorated);
        return decorated;
      })
    : decorateKnowledgeMemories(memoriesQuery.data, decorateOptions);

  const visibleMemories = listMemories;

  const selectedMemoryStillVisible =
    selectedMemoryKey !== null &&
    visibleMemories.some(memory => knowledgeMemoryKey(memory) === selectedMemoryKey);
  const effectiveSelectedMemoryKey = selectedMemoryStillVisible
    ? selectedMemoryKey
    : visibleMemories[0]
      ? knowledgeMemoryKey(visibleMemories[0])
      : null;

  const selectedMemory = visibleMemories.find(
    memory => knowledgeMemoryKey(memory) === effectiveSelectedMemoryKey
  );

  const detailSelector = selectedMemory ? selectorFromMemory(selectedMemory) : null;
  const memoryDetailQuery = useMemory(detailSelector ?? undefined, selectedMemory?.filename, {
    enabled: Boolean(detailSelector && selectedMemory),
  });
  const decisionsQuery = useMemoryDecisions(
    detailSelector && selectedMemory
      ? { ...detailSelector, filename: selectedMemory.filename, limit: 10 }
      : undefined,
    { enabled: Boolean(detailSelector && selectedMemory) }
  );

  const isListLoading = searchEnabled ? searchQueryResult.isLoading : memoriesQuery.isLoading;
  const listError = searchEnabled ? searchQueryResult.error : memoriesQuery.error;

  const isLoading = isListLoading && visibleMemories.length === 0;
  const error = visibleMemories.length === 0 ? (listError ?? null) : null;
  const listRetryError = visibleMemories.length > 0 ? (listError ?? null) : null;
  const retryMemories = () => {
    if (memoriesQuery.isFetchNextPageError) {
      void memoriesQuery.fetchNextPage();
      return;
    }
    void memoriesQuery.refetch();
  };
  const retryKnowledgeList = () => {
    if (searchEnabled) {
      void searchQueryResult.refetch();
      return;
    }
    retryMemories();
  };

  const decisionsForSelected: MemoryDecision[] = decisionsQuery.data?.decisions ?? [];

  const clearActionState = () => {
    if (actionTargetKey !== null || deleteMutationError !== null) {
      resetDeleteMutation();
    }
    if (editMutationError !== null) {
      resetEditMutation();
    }
    if (writeMutationError !== null) {
      resetWriteMutation();
    }
    if (revertMutationError !== null) {
      resetRevertMutation();
    }
    setActionTargetKey(null);
    setRevertingDecisionId(null);
  };

  const handleSetActiveScope = (nextScope: KnowledgeScope) => {
    clearActionState();
    setActiveScope(nextScope);
  };

  const handleSetAgentName = (next: string) => {
    clearActionState();
    setAgentName(next);
  };

  const handleSetAgentTier = (next: KnowledgeAgentTier) => {
    clearActionState();
    setAgentTier(next);
  };

  const handleSetSearchQuery = (next: string) => {
    clearActionState();
    setSearchQuery(next);
  };

  const handleSetSelectedMemoryKey = (next: string | null) => {
    clearActionState();
    setSelectedMemoryKey(next);
  };

  const handleSetCreateOpen = (next: boolean) => {
    if (next) {
      resetWriteMutation();
    }
    setCreateOpen(next);
  };

  const handleCreate = async (input: {
    type: MemoryType;
    name: string;
    description?: string;
    content: string;
  }) => {
    if (!selector) {
      return;
    }
    resetWriteMutation();
    const body: MemoryWriteRequest = {
      scope: selector.scope,
      type: input.type,
      name: input.name,
      description: input.description,
      content: input.content,
      workspace_id: selector.workspaceId,
      agent_name: selector.agentName,
      agent_tier: selector.agentTier,
    };
    const response = await writeMemoryMutate(body);
    const filename = response.decision.target_filename ?? response.decision.frontmatter.filename;
    setSearchQuery("");
    setSelectedMemoryKey(`${selector.scope}:${filename}`);
    setCreateOpen(false);
  };

  const handleDelete = async (memory: KnowledgeMemoryItem) => {
    const memorySelector = selectorFromMemory(memory);
    if (memorySelector.scope === "workspace" && !memorySelector.workspaceId) {
      return;
    }
    const memoryKey = knowledgeMemoryKey(memory);
    resetDeleteMutation();
    setActionTargetKey(memoryKey);
    await deleteMemoryMutate({ selector: memorySelector, filename: memory.filename });
    setActionTargetKey(prev => (prev === memoryKey ? null : prev));
  };

  const handleEdit = async (
    memory: KnowledgeMemoryItem,
    input: { content: string; description?: string }
  ) => {
    const memoryKey = knowledgeMemoryKey(memory);
    resetEditMutation();
    setActionTargetKey(memoryKey);
    // `name` and `type` are create-only identity: they key retrieval, so the edit
    // path renders them through `ImmutableIdentity` and omits them from the PATCH.
    const body: MemoryEditRequest = {
      content: input.content,
      description: input.description,
      scope: memory.scope,
      workspace_id: memory.workspace_id,
      agent_name: memory.agent_name,
      agent_tier: memory.agent_tier,
    };
    const params: EditMemoryParams = { filename: memory.filename, body };
    await editMemoryMutate(params);
    setActionTargetKey(prev => (prev === memoryKey ? null : prev));
  };

  const handleRevertDecision = async (decision: MemoryDecision) => {
    if (isRevertPending || revertingDecisionId !== null) {
      return;
    }

    resetRevertMutation();
    setRevertingDecisionId(decision.id);
    try {
      await revertMemoryDecisionMutate({
        decisionID: decision.id,
        body: { reason: "operator reverted from Knowledge" },
      });
      const filename = decision.target_filename ?? decision.frontmatter.filename;
      setSelectedMemoryKey(`${decision.scope}:${filename}`);
    } catch (error) {
      setRevertingDecisionId(prev => (prev === decision.id ? null : prev));
      throw error;
    }
    setRevertingDecisionId(prev => (prev === decision.id ? null : prev));
  };

  const selectedTargetMatches = selectedMemory
    ? actionTargetKey === knowledgeMemoryKey(selectedMemory)
    : false;

  const deleteError =
    selectedTargetMatches && deleteMutationError
      ? describeError(deleteMutationError, "Failed to delete knowledge entry")
      : null;
  const editError =
    selectedTargetMatches && editMutationError
      ? describeError(editMutationError, "Failed to edit knowledge entry")
      : null;
  const createError = writeMutationError
    ? describeError(writeMutationError, "Failed to create knowledge entry")
    : null;
  const revertError = revertMutationError
    ? describeError(revertMutationError, "Failed to revert memory decision")
    : null;

  const requiresWorkspace = activeScope === "workspace" && !activeWorkspaceId;
  const requiresAgentName = activeScope === "agent" && trimmedAgentName === "";
  const guardMessage = requiresWorkspace
    ? "Select an active workspace to view workspace memories."
    : requiresAgentName
      ? "Enter an agent name to view agent-scoped memories."
      : null;

  const searchInfo = searchEnabled
    ? `Recall ${searchQueryResult.data?.results.length ?? 0} of top-K`
    : null;

  return {
    activeScope,
    setActiveScope: handleSetActiveScope,
    agentName,
    setAgentName: handleSetAgentName,
    agentTier,
    setAgentTier: handleSetAgentTier,
    searchQuery,
    setSearchQuery: handleSetSearchQuery,
    setSelectedMemoryKey: handleSetSelectedMemoryKey,
    effectiveSelectedMemoryKey,
    memories: visibleMemories,
    memoryCount: searchEnabled ? visibleMemories.length : memoriesQuery.total,
    hasMoreMemories: !searchEnabled && memoriesQuery.hasNextPage,
    isLoadingMoreMemories: memoriesQuery.isFetchingNextPage,
    loadMoreMemories: () => {
      void memoriesQuery.fetchNextPage();
    },
    retryMemories,
    retryKnowledgeList,
    listRetryError,
    isLoading,
    error,
    selectedMemory,
    selectedScope: selectedMemory?.scope,
    selectedContent: memoryDetailQuery.data?.content,
    isContentLoading: memoryDetailQuery.isLoading && Boolean(selectedMemory),
    contentError: memoryDetailQuery.error,
    handleDelete,
    isDeletePending,
    deleteError,
    handleEdit,
    isEditPending,
    editError,
    createOpen,
    setCreateOpen: handleSetCreateOpen,
    handleCreate,
    isCreatePending,
    createError,
    createDefaultType: defaultMemoryType(activeScope),
    canCreateMemory: Boolean(selector),
    decisions: decisionsForSelected,
    decisionsError: decisionsQuery.error,
    isDecisionsLoading: decisionsQuery.isLoading && Boolean(selectedMemory),
    handleRevertDecision,
    revertingDecisionId,
    isRevertPending,
    revertError,
    searchActive: searchEnabled,
    searchInfo,
    guardMessage,
  };
}

export { useKnowledgePage };

function defaultMemoryType(scope: KnowledgeScope): MemoryType {
  return scope === "workspace" ? "project" : "user";
}
