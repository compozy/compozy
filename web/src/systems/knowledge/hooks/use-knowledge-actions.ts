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
  profile?: string;
}

export function useEditMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ filename, body, profile }: EditMemoryParams) =>
      editMemory(filename, body, profile),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

export interface WriteMemoryParams {
  body: MemoryWriteRequest;
  profile?: string;
}

export function useWriteMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ body, profile }: WriteMemoryParams) => writeMemory(body, profile),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

interface RevertMemoryDecisionParams {
  decisionID: string;
  body?: MemoryDecisionRevertRequest;
  profile?: string;
}

export function useRevertMemoryDecision() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ decisionID, body, profile }: RevertMemoryDecisionParams) =>
      revertMemoryDecision(decisionID, body ?? {}, profile),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}

interface TriggerMemoryDreamParams {
  workspaceID?: string;
  profile?: string;
}

export function useTriggerMemoryDream() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ workspaceID, profile }: TriggerMemoryDreamParams) =>
      triggerMemoryDream(workspaceID, profile),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: knowledgeKeys.all });
    },
  });
}
