import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  disableSkill,
  enableSkill,
  installSkillMarketplace,
  removeSkillMarketplace,
  updateSkillMarketplace,
} from "../adapters/skill-api";
import { skillKeys } from "../lib/query-keys";
import type { SkillMarketplaceInstallRequest, SkillMarketplaceUpdateRequest } from "../types";

interface SkillActionParams {
  name: string;
  workspace: string;
}

function invalidateSkillQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  name: string,
  workspace: string
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: skillKeys.list(workspace) }),
    queryClient.invalidateQueries({ queryKey: skillKeys.detail(name, workspace) }),
    queryClient.invalidateQueries({ queryKey: skillKeys.content(name, workspace) }),
  ]);
}

function invalidateInstalled(queryClient: ReturnType<typeof useQueryClient>, workspace: string) {
  return queryClient.invalidateQueries({ queryKey: skillKeys.list(workspace) });
}

export function useEnableSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, workspace }: SkillActionParams) => enableSkill(name, workspace),
    onSettled: (_data, _error, { name, workspace }) =>
      invalidateSkillQueries(queryClient, name, workspace),
  });
}

export function useDisableSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, workspace }: SkillActionParams) => disableSkill(name, workspace),
    onSettled: (_data, _error, { name, workspace }) =>
      invalidateSkillQueries(queryClient, name, workspace),
  });
}

interface InstallSkillVariables {
  body: SkillMarketplaceInstallRequest;
  workspace: string;
}

export function useInstallSkillMarketplace() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ body }: InstallSkillVariables) => installSkillMarketplace(body),
    onSettled: (_data, _error, { workspace }) => invalidateInstalled(queryClient, workspace),
  });
}

interface UpdateSkillVariables {
  body: SkillMarketplaceUpdateRequest;
  workspace: string;
}

export function useUpdateSkillMarketplace() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ body }: UpdateSkillVariables) => updateSkillMarketplace(body),
    onSettled: (_data, _error, { workspace }) => invalidateInstalled(queryClient, workspace),
  });
}

interface RemoveSkillVariables {
  name: string;
  workspace: string;
}

export function useRemoveSkillMarketplace() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name }: RemoveSkillVariables) => removeSkillMarketplace(name),
    onSettled: (_data, _error, { workspace }) => invalidateInstalled(queryClient, workspace),
  });
}
