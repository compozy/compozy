import { queryOptions } from "@tanstack/react-query";

import { getSkill, getSkillContent, getSkillShadows, listSkills } from "../adapters/skill-api";
import { skillKeys } from "./query-keys";

export function skillsListOptions(workspace: string, enabled = true, profile?: string) {
  return queryOptions({
    queryKey: skillKeys.list(workspace, profile),
    queryFn: ({ signal }) =>
      profile === undefined
        ? listSkills(workspace, signal)
        : listSkills(workspace, signal, profile),
    staleTime: 30_000,
    refetchInterval: 60_000,
    enabled,
  });
}

export function skillDetailOptions(name: string, workspace: string, profile?: string) {
  return queryOptions({
    queryKey: skillKeys.detail(name, workspace, profile),
    queryFn: ({ signal }) =>
      profile === undefined
        ? getSkill(name, workspace, signal)
        : getSkill(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function skillContentOptions(
  name: string,
  workspace: string,
  enabled: boolean,
  profile?: string
) {
  return queryOptions({
    queryKey: skillKeys.content(name, workspace, profile),
    queryFn: ({ signal }) =>
      profile === undefined
        ? getSkillContent(name, workspace, signal)
        : getSkillContent(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: enabled && !!name,
  });
}

export function skillShadowsOptions(name: string, workspace: string, profile?: string) {
  return queryOptions({
    queryKey: skillKeys.shadows(name, workspace, profile),
    queryFn: ({ signal }) =>
      profile === undefined
        ? getSkillShadows(name, workspace, signal)
        : getSkillShadows(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}
