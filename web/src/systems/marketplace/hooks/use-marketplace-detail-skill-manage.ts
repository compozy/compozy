import { useState } from "react";

import {
  useDisableSkill,
  useEnableSkill,
  useSkill,
  useSkillContent,
  useSkillShadows,
} from "@/systems/skill";
import { useActiveWorkspace } from "@/systems/workspace";
import { useProfileReadScope } from "@/systems/profiles";

function useMarketplaceDetailSkillManage(name: string) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const { destination } = useProfileReadScope();
  const workspaceId = activeWorkspaceId ?? "";
  const profile = destination === "default" ? undefined : destination;
  const [contentRequested, setContentRequested] = useState(false);
  const skillQuery = useSkill(name, workspaceId, profile);
  const contentQuery = useSkillContent(name, workspaceId, contentRequested, profile);
  const shadowsQuery = useSkillShadows(name, workspaceId, profile);
  const enableMutation = useEnableSkill();
  const disableMutation = useDisableSkill();
  const skill = skillQuery.data;
  const acting = enableMutation.isPending || disableMutation.isPending;
  const toggleMutationError = enableMutation.error ?? disableMutation.error;
  const toggleError = toggleMutationError
    ? toggleMutationError instanceof Error && toggleMutationError.message.trim().length > 0
      ? toggleMutationError.message
      : "Couldn't update skill availability"
    : null;

  return {
    acting,
    contentQuery,
    contentRequested,
    setContentRequested,
    shadowsQuery,
    skill,
    skillQuery,
    toggleError,
    toggleEnabled: (next: boolean) => {
      if (acting || !skill) return;
      if (next) enableMutation.mutate({ name, workspace: workspaceId, profile });
      else disableMutation.mutate({ name, workspace: workspaceId, profile });
    },
  };
}

export { useMarketplaceDetailSkillManage };
