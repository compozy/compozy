import { useQuery } from "@tanstack/react-query";

import {
  skillContentOptions,
  skillDetailOptions,
  skillShadowsOptions,
  skillsListOptions,
} from "@/systems/skill/lib/query-options";

export function useSkills(workspace: string, enabled = true, profile?: string) {
  return useQuery(skillsListOptions(workspace, enabled, profile));
}

export function useSkill(name: string, workspace: string, profile?: string) {
  return useQuery(skillDetailOptions(name, workspace, profile));
}

export function useSkillContent(
  name: string,
  workspace: string,
  enabled = false,
  profile?: string
) {
  return useQuery(skillContentOptions(name, workspace, enabled, profile));
}

export function useSkillShadows(name: string, workspace: string, profile?: string) {
  return useQuery(skillShadowsOptions(name, workspace, profile));
}
