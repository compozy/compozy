import { useMutation, useQueryClient } from "@tanstack/react-query";

import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
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
  computerId?: ComputerId;
}

export type CreateAutomationJobVariables = CreateAutomationJobRequest & {
  computerId?: ComputerId;
};

interface UpdateAutomationJobParams extends AutomationIdParams {
  data: UpdateAutomationJobRequest;
}

interface UpdateAutomationTriggerParams extends AutomationIdParams {
  data: UpdateAutomationTriggerRequest;
}

function invalidateJobQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  id?: string,
  computerId?: ComputerId
) {
  const keys = automationKeys.forComputer(normalizeComputerId(computerId));
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.jobs() }),
    queryClient.invalidateQueries({ queryKey: keys.runs() }),
    ...(id
      ? [
          queryClient.invalidateQueries({ queryKey: keys.jobDetail(id) }),
          queryClient.invalidateQueries({ queryKey: keys.jobRunsFor(id) }),
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
    mutationFn: ({ computerId, ...data }: CreateAutomationJobVariables) =>
      computerId
        ? createAutomationJob(data, profile.destination, undefined, computerId)
        : createAutomationJob(data, profile.destination),
    onSuccess: job => profile.notifyCreated(job.profile_name),
    onSettled: (_result, _error, variables) =>
      invalidateJobQueries(queryClient, undefined, variables.computerId),
  });
}

export function useUpdateAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data, profile, computerId }: UpdateAutomationJobParams) =>
      computerId
        ? updateAutomationJob(id, data, profile, undefined, computerId)
        : updateAutomationJob(id, data, profile),
    onSettled: (_result, _error, { id, computerId }) =>
      invalidateJobQueries(queryClient, id, computerId),
  });
}

export function useDeleteAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, profile, computerId }: AutomationIdParams) =>
      computerId
        ? deleteAutomationJob(id, profile, undefined, computerId)
        : deleteAutomationJob(id, profile),
    onSettled: (_result, _error, { id, computerId }) =>
      invalidateJobQueries(queryClient, id, computerId),
  });
}

export function useTriggerAutomationJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, profile, computerId }: AutomationIdParams) =>
      computerId
        ? triggerAutomationJob(id, profile, undefined, computerId)
        : triggerAutomationJob(id, profile),
    onSettled: (_result, _error, { id, computerId }) =>
      invalidateJobQueries(queryClient, id, computerId),
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
