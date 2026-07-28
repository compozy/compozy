import { useQuery } from "@tanstack/react-query";

import {
  skillContentOptions,
  skillDetailOptions,
  skillShadowsOptions,
  skillsListOptions,
} from "@/systems/skill/lib/query-options";

export function useSkills(workspace: string) {
  return useQuery(skillsListOptions(workspace));
}

export function useSkill(name: string, workspace: string) {
  return useQuery(skillDetailOptions(name, workspace));
}

export function useSkillContent(name: string, workspace: string, enabled = false) {
  return useQuery(skillContentOptions(name, workspace, enabled));
}

export function useSkillShadows(name: string, workspace: string) {
  return useQuery(skillShadowsOptions(name, workspace));
}
