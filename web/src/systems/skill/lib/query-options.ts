import { queryOptions } from "@tanstack/react-query";

import { getSkill, getSkillContent, getSkillShadows, listSkills } from "../adapters/skill-api";
import { skillKeys } from "./query-keys";

export function skillsListOptions(workspace: string) {
  return queryOptions({
    queryKey: skillKeys.list(workspace),
    queryFn: ({ signal }) => listSkills(workspace, signal),
    staleTime: 30_000,
    refetchInterval: 60_000,
    enabled: true,
  });
}

export function skillDetailOptions(name: string, workspace: string) {
  return queryOptions({
    queryKey: skillKeys.detail(name, workspace),
    queryFn: ({ signal }) => getSkill(name, workspace, signal),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function skillContentOptions(name: string, workspace: string, enabled: boolean) {
  return queryOptions({
    queryKey: skillKeys.content(name, workspace),
    queryFn: ({ signal }) => getSkillContent(name, workspace, signal),
    staleTime: 30_000,
    enabled: enabled && !!name,
  });
}

export function skillShadowsOptions(name: string, workspace: string) {
  return queryOptions({
    queryKey: skillKeys.shadows(name, workspace),
    queryFn: ({ signal }) => getSkillShadows(name, workspace, signal),
    staleTime: 30_000,
    enabled: !!name,
  });
}
