import { useMutation, useQueryClient } from "@tanstack/react-query";

import { disableSkill, enableSkill } from "../adapters/skill-api";

import { skillKeys } from "../lib/query-keys";
import { sessionKeys } from "@/systems/session";

interface SkillActionParams {
  name: string;
  workspace: string;
  profile?: string;
}

function invalidateSkillQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  name: string,
  workspace: string,
  profile?: string
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: skillKeys.list(workspace, profile) }),
    queryClient.invalidateQueries({ queryKey: skillKeys.detail(name, workspace, profile) }),
    queryClient.invalidateQueries({ queryKey: skillKeys.content(name, workspace, profile) }),
    queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceCommands(workspace) }),
  ]);
}

export function useEnableSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, workspace, profile }: SkillActionParams) =>
      enableSkill(name, workspace, profile),
    onSettled: (_data, _error, { name, workspace, profile }) =>
      invalidateSkillQueries(queryClient, name, workspace, profile),
  });
}

export function useDisableSkill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, workspace, profile }: SkillActionParams) =>
      disableSkill(name, workspace, profile),
    onSettled: (_data, _error, { name, workspace, profile }) =>
      invalidateSkillQueries(queryClient, name, workspace, profile),
  });
}
