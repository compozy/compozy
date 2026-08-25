import { useSettingsSkills } from "@/systems/settings";
import { useProfileReadScope } from "@/systems/profiles";

import { isSkillExposable, skillExposureViews } from "../lib/skill-exposure-view";
import type { SkillExposureView } from "../lib/skill-exposure-view";
import type { SkillPayload } from "../types";
import type { SkillExposeTarget } from "../components/skill-expose-target-picker";
import { useSkillExpose, type SkillExposeModel } from "./use-skill-expose";

export interface SkillExposuresModel {
  /** False when the skill has no on-disk home to link; the panel is then absent. */
  eligible: boolean;
  exposures: SkillExposureView[];
  targets: SkillExposeTarget[];
  targetsLoading: boolean;
  targetsError: string | null;
  retryTargets: () => void;
  expose: SkillExposeModel;
  labelForTarget: (slug: string) => string;
}

/**
 * Exposure state for one skill, plus the sources it could be exposed into.
 *
 * Targets are the daemon's enabled presets for this workspace — custom folders
 * are excluded because a link into a machine-local path would travel to
 * teammates as a broken one.
 */
export function useSkillExposures(skill: SkillPayload, workspaceId: string): SkillExposuresModel {
  const { destination } = useProfileReadScope();
  const profile = destination === "default" ? undefined : destination;
  const settings = useSettingsSkills(
    profile
      ? { scope: "profile", profile, ...(workspaceId ? { workspace_id: workspaceId } : {}) }
      : workspaceId === ""
        ? {}
        : { scope: "workspace", workspace_id: workspaceId }
  );
  const expose = useSkillExpose(skill.name, workspaceId, profile);
  const sources = settings.data?.sources ?? [];
  const targets: SkillExposeTarget[] = [];
  const labelBySlug = new Map<string, string>();
  for (const source of sources) {
    labelBySlug.set(source.slug, source.label);
    if (source.kind !== "preset" || !source.enabled || source.slug === skill.origin) continue;
    targets.push({
      slug: source.slug,
      label: source.label,
      hint: source.workspace_path ?? source.global_path ?? null,
    });
  }

  return {
    eligible: isSkillExposable(skill),
    exposures: skillExposureViews(skill),
    targets,
    targetsLoading: settings.isLoading,
    targetsError: settings.error instanceof Error ? settings.error.message : null,
    retryTargets: () => {
      void settings.refetch();
    },
    expose,
    labelForTarget: slug => labelBySlug.get(slug) ?? slug,
  };
}
