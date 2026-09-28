import { useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";

import {
  type KnowledgeAgentTier,
  knowledgeMemoryKey,
  type KnowledgeScope,
  type MemoryDecision,
  useMemory,
  useMemoryDecisions,
} from "@/systems/knowledge";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace, useCreateDestination } from "@/systems/workspace";

import { knowledgeGuard } from "./knowledge-page-copy";
import {
  buildCreateSelector,
  buildListSelector,
  createDefaultType,
  type DecorateOptions,
  resolveProjectName,
  selectorFromMemory,
} from "./knowledge-page-model";
import { resolveKnowledgeSelectedKey } from "./knowledge-route-selection";
import {
  useKnowledgeCreate,
  useKnowledgeEntryActions,
  useKnowledgeRevert,
} from "./use-knowledge-page-actions";
import { useKnowledgeList } from "./use-knowledge-list";
import { type KnowledgeRouteOptions, useKnowledgeRouteState } from "./use-knowledge-route-state";

function useKnowledgePage(options?: KnowledgeRouteOptions) {
  const { activeWorkspaceId, data: knownWorkspaces } = useActiveWorkspace();
  const destination = useCreateDestination();
  const { destination: profile } = useProfileReadScope();
  const route = useKnowledgeRouteState(options);
  const { activeScope, setActiveScope, setSelectedMemoryKey } = route;

  const [agentName, setAgentName] = useState("");
  const [agentTier, setAgentTier] = useState<KnowledgeAgentTier>("workspace");
  const searchInput = useDebouncedInput({
    externalValue: "",
    onCommit: () => undefined,
  });

  const trimmedAgentName = agentName.trim();
  const isAgentScope = activeScope === "agent";
  const listWorkspaceId = route.routeWorkspaceId || activeWorkspaceId;
  const selector = buildListSelector({
    profile,
    activeScope,
    listWorkspaceId,
    activeWorkspaceId,
    agentName: trimmedAgentName,
    agentTier,
  });
  const createSelector = buildCreateSelector(activeScope, selector, destination, profile);
  const decorateOptions: DecorateOptions = {
    scope: activeScope,
    agentTier: isAgentScope ? agentTier : undefined,
    agentName: isAgentScope ? trimmedAgentName : undefined,
    workspaceId: selector?.workspaceId,
  };

  const list = useKnowledgeList(selector, searchInput.committedValue.trim(), decorateOptions);
  const effectiveSelectedMemoryKey = resolveKnowledgeSelectedKey(
    route.routeMemory,
    route.selectedMemoryKey,
    list.memories
  );
  const selectedMemory = list.memories.find(
    memory => knowledgeMemoryKey(memory) === effectiveSelectedMemoryKey
  );
  const hasSelection = selectedMemory !== undefined;
  const detailSelector = selectedMemory ? selectorFromMemory(selectedMemory, profile) : undefined;
  const memoryDetailQuery = useMemory(detailSelector, selectedMemory?.filename, {
    enabled: hasSelection,
  });
  const decisionsQuery = useMemoryDecisions(
    detailSelector && { ...detailSelector, filename: selectedMemory?.filename, limit: 10 },
    { enabled: hasSelection }
  );

  const entryActions = useKnowledgeEntryActions(profile, selectedMemory);
  const create = useKnowledgeCreate({
    createSelector,
    onCreated: (scope, key) => {
      searchInput.clear();
      if (!isAgentScope) {
        setActiveScope(scope);
      }
      setSelectedMemoryKey(key);
    },
  });
  const revert = useKnowledgeRevert(profile, setSelectedMemoryKey);

  const withClearedActions =
    <T>(apply: (next: T) => void) =>
    (next: T) => {
      entryActions.clear();
      create.clear();
      revert.clear();
      apply(next);
    };

  const decisions: MemoryDecision[] = decisionsQuery.data?.decisions ?? [];

  return {
    activeScope,
    setActiveScope: withClearedActions<KnowledgeScope>(setActiveScope),
    agentName,
    setAgentName: withClearedActions<string>(setAgentName),
    agentTier,
    setAgentTier: withClearedActions<KnowledgeAgentTier>(setAgentTier),
    searchQuery: searchInput.draftValue,
    setSearchQuery: withClearedActions<string>(searchInput.setDraftValue),
    setSelectedMemoryKey: withClearedActions<string | null>(setSelectedMemoryKey),
    effectiveSelectedMemoryKey,
    ...list,
    selectedMemory,
    selectedScope: selectedMemory?.scope,
    selectedProjectName: resolveProjectName(
      selectedMemory?.workspace_id,
      knownWorkspaces,
      activeWorkspaceId
    ),
    selectedContent: memoryDetailQuery.data?.content,
    isContentLoading: memoryDetailQuery.isLoading && hasSelection,
    contentError: memoryDetailQuery.error,
    handleDelete: entryActions.handleDelete,
    isDeletePending: entryActions.isDeletePending,
    deleteError: entryActions.deleteError,
    handleEdit: entryActions.handleEdit,
    isEditPending: entryActions.isEditPending,
    editError: entryActions.editError,
    createOpen: create.createOpen,
    setCreateOpen: create.setCreateOpen,
    handleCreate: create.handleCreate,
    isCreatePending: create.isCreatePending,
    createError: create.createError,
    createDefaultType: createDefaultType(createSelector),
    canCreateMemory: Boolean(createSelector),
    createDestinationLabel: isAgentScope
      ? trimmedAgentName || "Agent"
      : destination.destinationLabel,
    createScope: createSelector?.scope ?? "profile",
    decisions,
    decisionsError: decisionsQuery.error,
    isDecisionsLoading: decisionsQuery.isLoading && hasSelection,
    handleRevertDecision: revert.handleRevertDecision,
    revertingDecisionId: revert.revertingDecisionId,
    isRevertPending: revert.isRevertPending,
    revertError: revert.revertError,
    guard: knowledgeGuard({
      requiresWorkspace: activeScope === "workspace" && !listWorkspaceId,
      requiresAgentName: isAgentScope && trimmedAgentName === "",
    }),
  };
}

export { useKnowledgePage };
