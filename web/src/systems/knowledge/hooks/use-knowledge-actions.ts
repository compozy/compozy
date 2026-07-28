import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  deleteMemory,
  editMemory,
  revertMemoryDecision,
  triggerMemoryDream,
  writeMemory,
} from "@/systems/knowledge/adapters/knowledge-api";
import { knowledgeKeys } from "@/systems/knowledge/lib/query-keys";
import type {
  KnowledgeSelector,
  MemoryDecisionRevertRequest,
  MemoryEditRequest,
  MemoryWriteRequest,
} from "@/systems/knowledge/types";

interface DeleteMemoryParams {
  selector: KnowledgeSelector;
  filename: string;
}

export function useDeleteMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ selector, filename }: DeleteMemoryParams) => deleteMemory(selector, filename),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

export interface EditMemoryParams {
  filename: string;
  body: MemoryEditRequest;
}

export function useEditMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ filename, body }: EditMemoryParams) => editMemory(filename, body),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

export function useWriteMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: MemoryWriteRequest) => writeMemory(body),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

interface RevertMemoryDecisionParams {
  decisionID: string;
  body?: MemoryDecisionRevertRequest;
}

export function useRevertMemoryDecision() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ decisionID, body }: RevertMemoryDecisionParams) =>
      revertMemoryDecision(decisionID, body ?? {}),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

interface TriggerMemoryDreamParams {
  workspaceID?: string;
}

export function useTriggerMemoryDream() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ workspaceID }: TriggerMemoryDreamParams) => triggerMemoryDream(workspaceID),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}
