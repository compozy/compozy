import { useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";

import {
  DEFAULT_MEMORY_LIST_LIMIT,
  type EditMemoryParams,
  type KnowledgeAgentTier,
  type KnowledgeListFilter,
  type KnowledgeMemoryItem,
  knowledgeMemoryKey,
  type KnowledgeScope,
  type KnowledgeSelector,
  type MemoryDecision,
  type MemoryEditRequest,
  type MemoryHeader,
  type MemoryType,
  type MemoryWriteRequest,
  useDeleteMemory,
  useEditMemory,
  useMemories,
  useMemory,
  useMemoryDecisions,
  useMemorySearch,
  useRevertMemoryDecision,
  useWriteMemory,
} from "@/systems/knowledge";
import { useActiveWorkspace, useCreateDestination } from "@/systems/workspace";

import { resolveKnowledgeSelectedKey } from "./knowledge-route-selection";

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

function useKnowledgePage(options?: {
  routeMemory?: string | null;
  routeScope?: KnowledgeScope | null;
  routeWorkspaceId?: string | null;
}) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const destination = useCreateDestination();

  const [activeScope, setActiveScope] = useState<KnowledgeScope>(options?.routeScope ?? "global");
  const [agentName, setAgentName] = useState("");
  const [agentTier, setAgentTier] = useState<KnowledgeAgentTier>("workspace");
  const searchInput = useDebouncedInput({
    externalValue: "",
    onCommit: () => undefined,
  });
  const searchQuery = searchInput.draftValue;
  const routeMemory = options?.routeMemory?.trim() || null;
  const routeScope = options?.routeScope ?? null;
  const routeWorkspaceId = options?.routeWorkspaceId?.trim() || null;
  const [appliedRoute, setAppliedRoute] = useState({
    memory: routeMemory,
    scope: routeScope,
    workspace: routeWorkspaceId,
  });
  const [selectedMemoryKey, setSelectedMemoryKey] = useState<string | null>(null);
  if (
    routeMemory !== appliedRoute.memory ||
    routeScope !== appliedRoute.scope ||
    routeWorkspaceId !== appliedRoute.workspace
  ) {
    setAppliedRoute({ memory: routeMemory, scope: routeScope, workspace: routeWorkspaceId });
    setSelectedMemoryKey(null);
    if (routeScope !== null) {
      setActiveScope(routeScope);
    }
  }
  const [actionTargetKey, setActionTargetKey] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [revertingDecisionId, setRevertingDecisionId] = useState<string | null>(null);

  const trimmedAgentName = agentName.trim();
  const trimmedSearchQuery = searchInput.committedValue.trim();

  const listWorkspaceId = routeWorkspaceId || activeWorkspaceId;
  let selector: KnowledgeSelector | null = { scope: "global" };
  if (activeScope === "workspace") {
    selector = listWorkspaceId ? { scope: "workspace", workspaceId: listWorkspaceId } : null;
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

  const createSelector: KnowledgeSelector | null =
    activeScope === "agent"
      ? selector
      : destination.scope === "workspace" && destination.workspaceId
        ? { scope: "workspace", workspaceId: destination.workspaceId }
        : { scope: "global" };
  const createDestinationLabel =
    activeScope === "agent" ? trimmedAgentName || "Agent" : destination.destinationLabel;

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

  const effectiveSelectedMemoryKey = resolveKnowledgeSelectedKey(
    routeMemory,
    selectedMemoryKey,
    visibleMemories
  );

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
    searchInput.setDraftValue(next);
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
    if (!createSelector) {
      return;
    }
    resetWriteMutation();
    const body: MemoryWriteRequest = {
      scope: createSelector.scope,
      type: input.type,
      name: input.name,
      description: input.description,
      content: input.content,
      workspace_id: createSelector.workspaceId,
      agent_name: createSelector.agentName,
      agent_tier: createSelector.agentTier,
    };
    const response = await writeMemoryMutate(body);
    const filename = response.decision.target_filename ?? response.decision.frontmatter.filename;
    searchInput.clear();
    if (activeScope !== "agent") {
      setActiveScope(createSelector.scope);
    }
    setSelectedMemoryKey(`${createSelector.scope}:${filename}`);
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

  const requiresWorkspace = activeScope === "workspace" && !listWorkspaceId;
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
    createDefaultType: defaultMemoryType(
      createSelector?.scope === "workspace"
        ? "workspace"
        : activeScope === "agent"
          ? "agent"
          : "global"
    ),
    canCreateMemory: Boolean(createSelector),
    createDestinationLabel,
    createScope: createSelector?.scope ?? "global",
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
