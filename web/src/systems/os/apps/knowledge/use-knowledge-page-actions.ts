import { useState } from "react";

import {
  type EditMemoryParams,
  type KnowledgeMemoryItem,
  knowledgeMemoryKey,
  type KnowledgeScope,
  type KnowledgeSelector,
  type MemoryDecision,
  type MemoryEditRequest,
  type MemoryType,
  type MemoryWriteRequest,
  useDeleteMemory,
  useEditMemory,
  useRevertMemoryDecision,
  useWriteMemory,
} from "@/systems/knowledge";

import { describeError, selectorFromMemory } from "./knowledge-page-model";

interface KnowledgeCreateInput {
  type: MemoryType;
  name: string;
  description?: string;
  content: string;
}

/** Delete and edit share one in-flight target so errors only surface on that entry. */
function useKnowledgeEntryActions(profile: string, selectedMemory?: KnowledgeMemoryItem) {
  const [actionTargetKey, setActionTargetKey] = useState<string | null>(null);
  const deleteMutation = useDeleteMemory();
  const editMutation = useEditMemory();

  const releaseTarget = (memoryKey: string) => {
    setActionTargetKey(prev => (prev === memoryKey ? null : prev));
  };

  const handleDelete = async (memory: KnowledgeMemoryItem) => {
    const memorySelector = selectorFromMemory(memory, profile);
    if (memorySelector.scope === "workspace" && !memorySelector.workspaceId) {
      return;
    }
    const memoryKey = knowledgeMemoryKey(memory);
    deleteMutation.reset();
    setActionTargetKey(memoryKey);
    await deleteMutation.mutateAsync({ selector: memorySelector, filename: memory.filename });
    releaseTarget(memoryKey);
  };

  const handleEdit = async (
    memory: KnowledgeMemoryItem,
    input: { content: string; description?: string }
  ) => {
    const memoryKey = knowledgeMemoryKey(memory);
    editMutation.reset();
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
    const params: EditMemoryParams = { filename: memory.filename, body, profile };
    await editMutation.mutateAsync(params);
    releaseTarget(memoryKey);
  };

  const clear = () => {
    if (actionTargetKey !== null || deleteMutation.error !== null) {
      deleteMutation.reset();
    }
    if (editMutation.error !== null) {
      editMutation.reset();
    }
    setActionTargetKey(null);
  };

  const targetsSelected =
    selectedMemory !== undefined && actionTargetKey === knowledgeMemoryKey(selectedMemory);

  return {
    clear,
    handleDelete,
    isDeletePending: deleteMutation.isPending,
    deleteError: targetsSelected
      ? describeError(deleteMutation.error, "Failed to delete knowledge entry")
      : null,
    handleEdit,
    isEditPending: editMutation.isPending,
    editError: targetsSelected
      ? describeError(editMutation.error, "Failed to edit knowledge entry")
      : null,
  };
}

interface KnowledgeCreateOptions {
  createSelector: KnowledgeSelector | null;
  onCreated: (scope: KnowledgeScope, key: string) => void;
}

function useKnowledgeCreate({ createSelector, onCreated }: KnowledgeCreateOptions) {
  const [createOpen, setCreateOpen] = useState(false);
  const writeMutation = useWriteMemory();

  const handleSetCreateOpen = (next: boolean) => {
    if (next) {
      writeMutation.reset();
    }
    setCreateOpen(next);
  };

  const handleCreate = async (input: KnowledgeCreateInput) => {
    if (!createSelector) {
      return;
    }
    writeMutation.reset();
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
    const response = await writeMutation.mutateAsync({ body, profile: createSelector.profile });
    const filename = response.decision.target_filename ?? response.decision.frontmatter.filename;
    onCreated(createSelector.scope, `${createSelector.scope}:${filename}`);
    setCreateOpen(false);
  };

  const clear = () => {
    if (writeMutation.error !== null) {
      writeMutation.reset();
    }
  };

  return {
    clear,
    createOpen,
    setCreateOpen: handleSetCreateOpen,
    handleCreate,
    isCreatePending: writeMutation.isPending,
    createError: describeError(writeMutation.error, "Failed to create knowledge entry"),
  };
}

/** Reverts one decision at a time and reselects the entry it restored. */
function useKnowledgeRevert(profile: string, onReverted: (key: string) => void) {
  const [revertingDecisionId, setRevertingDecisionId] = useState<string | null>(null);
  const revertMutation = useRevertMemoryDecision();

  const release = (decisionId: string) => {
    setRevertingDecisionId(prev => (prev === decisionId ? null : prev));
  };

  const handleRevertDecision = async (decision: MemoryDecision) => {
    if (revertMutation.isPending || revertingDecisionId !== null) {
      return;
    }
    revertMutation.reset();
    setRevertingDecisionId(decision.id);
    try {
      await revertMutation.mutateAsync({
        decisionID: decision.id,
        body: { reason: "operator reverted from Knowledge" },
        profile,
      });
      const filename = decision.target_filename ?? decision.frontmatter.filename;
      onReverted(`${decision.scope}:${filename}`);
    } catch (error) {
      release(decision.id);
      throw error;
    }
    release(decision.id);
  };

  const clear = () => {
    if (revertMutation.error !== null) {
      revertMutation.reset();
    }
    setRevertingDecisionId(null);
  };

  return {
    clear,
    handleRevertDecision,
    revertingDecisionId,
    isRevertPending: revertMutation.isPending,
    revertError: describeError(revertMutation.error, "Failed to revert memory decision"),
  };
}

export { useKnowledgeCreate, useKnowledgeEntryActions, useKnowledgeRevert };
export type { KnowledgeCreateInput };
