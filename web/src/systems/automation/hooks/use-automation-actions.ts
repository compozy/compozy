import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  createAutomationJob,
  createAutomationTrigger,
  deleteAutomationJob,
  deleteAutomationTrigger,
  triggerAutomationJob,
  updateAutomationJob,
  updateAutomationTrigger,
} from "../adapters/automation-api";
import { automationKeys } from "../lib/query-keys";
import type {
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
  UpdateAutomationJobRequest,
  UpdateAutomationTriggerRequest,
} from "../types";

interface AutomationIdParams {
  id: string;
}

interface UpdateAutomationJobParams extends AutomationIdParams {
  data: UpdateAutomationJobRequest;
}

interface UpdateAutomationTriggerParams extends AutomationIdParams {
  data: UpdateAutomationTriggerRequest;
}

function invalidateJobQueries(queryClient: ReturnType<typeof useQueryClient>, id?: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: automationKeys.jobs() }),
    queryClient.invalidateQueries({ queryKey: automationKeys.runs() }),
    ...(id
      ? [
          queryClient.invalidateQueries({ queryKey: automationKeys.jobDetail(id) }),
          queryClient.invalidateQueries({ queryKey: automationKeys.jobRuns(id) }),
        ]
      : []),
  ]);
}

function invalidateTriggerQueries(queryClient: ReturnType<typeof useQueryClient>, id?: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: automationKeys.triggers() }),
    queryClient.invalidateQueries({ queryKey: automationKeys.runs() }),
    ...(id
      ? [
          queryClient.invalidateQueries({ queryKey: automationKeys.triggerDetail(id) }),
          queryClient.invalidateQueries({ queryKey: automationKeys.triggerRuns(id) }),
        ]
      : []),
  ]);
}

export function useCreateAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAutomationJobRequest) => createAutomationJob(data),
    onSettled: () => invalidateJobQueries(queryClient),
  });
}

export function useUpdateAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: UpdateAutomationJobParams) => updateAutomationJob(id, data),
    onSettled: (_result, _error, { id }) => invalidateJobQueries(queryClient, id),
  });
}

export function useDeleteAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: AutomationIdParams) => deleteAutomationJob(id),
    onSettled: (_result, _error, { id }) => invalidateJobQueries(queryClient, id),
  });
}

export function useTriggerAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: AutomationIdParams) => triggerAutomationJob(id),
    onSettled: (_result, _error, { id }) => invalidateJobQueries(queryClient, id),
  });
}

export function useCreateAutomationTrigger() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAutomationTriggerRequest) => createAutomationTrigger(data),
    onSettled: () => invalidateTriggerQueries(queryClient),
  });
}

export function useUpdateAutomationTrigger() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: UpdateAutomationTriggerParams) => updateAutomationTrigger(id, data),
    onSettled: (_result, _error, { id }) => invalidateTriggerQueries(queryClient, id),
  });
}

export function useDeleteAutomationTrigger() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: AutomationIdParams) => deleteAutomationTrigger(id),
    onSettled: (_result, _error, { id }) => invalidateTriggerQueries(queryClient, id),
  });
}
