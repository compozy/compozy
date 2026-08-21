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
import { notifyUser } from "@/lib/user-feedback";
import { createdInProfileToast, useProfileReadScope } from "@/systems/profiles";

interface AutomationIdParams {
  id: string;
  profile: string;
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
          queryClient.invalidateQueries({ queryKey: automationKeys.jobRunsFor(id) }),
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
          queryClient.invalidateQueries({ queryKey: automationKeys.triggerRunsFor(id) }),
        ]
      : []),
  ]);
}

function useAutomationCreationProfile() {
  // Files into the acting profile — `default` while the aggregate is on (ADR-005).
  // The toast names the owner only there, where it is not already on screen.
  const { aggregate, destination } = useProfileReadScope();
  return {
    destination,
    notifyCreated: (profileName: string) => {
      if (aggregate) {
        notifyUser({ message: createdInProfileToast(profileName), tone: "success" });
      }
    },
  };
}

export function useCreateAutomationJob() {
  const queryClient = useQueryClient();
  const profile = useAutomationCreationProfile();

  return useMutation({
    mutationFn: (data: CreateAutomationJobRequest) =>
      createAutomationJob(data, profile.destination),
    onSuccess: job => profile.notifyCreated(job.profile_name),
    onSettled: () => invalidateJobQueries(queryClient),
  });
}

export function useUpdateAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data, profile }: UpdateAutomationJobParams) =>
      updateAutomationJob(id, data, profile),
    onSettled: (_result, _error, { id }) => invalidateJobQueries(queryClient, id),
  });
}

export function useDeleteAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, profile }: AutomationIdParams) => deleteAutomationJob(id, profile),
    onSettled: (_result, _error, { id }) => invalidateJobQueries(queryClient, id),
  });
}

export function useTriggerAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, profile }: AutomationIdParams) => triggerAutomationJob(id, profile),
    onSettled: (_result, _error, { id }) => invalidateJobQueries(queryClient, id),
  });
}

export function useCreateAutomationTrigger() {
  const queryClient = useQueryClient();
  const profile = useAutomationCreationProfile();

  return useMutation({
    mutationFn: (data: CreateAutomationTriggerRequest) =>
      createAutomationTrigger(data, profile.destination),
    onSuccess: trigger => profile.notifyCreated(trigger.profile_name),
    onSettled: () => invalidateTriggerQueries(queryClient),
  });
}

export function useUpdateAutomationTrigger() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data, profile }: UpdateAutomationTriggerParams) =>
      updateAutomationTrigger(id, data, profile),
    onSettled: (_result, _error, { id }) => invalidateTriggerQueries(queryClient, id),
  });
}

export function useDeleteAutomationTrigger() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, profile }: AutomationIdParams) => deleteAutomationTrigger(id, profile),
    onSettled: (_result, _error, { id }) => invalidateTriggerQueries(queryClient, id),
  });
}
