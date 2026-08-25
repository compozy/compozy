import type { StatusDotTone } from "@compozy/ui";

import type {
  SkillExposeResult,
  SkillExposurePayload,
  SkillExposureStatus,
  SkillPayload,
} from "../types";

/**
 * Presentation rules for expose links.
 *
 * The four statuses are not decoration: `missing` and `broken` are links
 * CompozyOS created and can repair, while `foreign_conflict` is someone else's
 * file at the same path. Repair affordances follow that ownership line exactly —
 * a foreign entry gets a sentence and nothing else.
 */

export interface SkillExposureView {
  target: string;
  path: string;
  status: SkillExposureStatus;
  /** Plain sentence; the status word alone never carries the meaning. */
  sentence: string;
  tone: StatusDotTone;
  /** True only for links CompozyOS owns and can recreate. */
  repairable: boolean;
  /** True when the link can be removed — never for a foreign entry. */
  removable: boolean;
  /** The path no longer resolves, so it renders struck through. */
  stale: boolean;
}

const STATUS_SENTENCE: Record<SkillExposureStatus, string> = {
  healthy: "active",
  missing: "the link was deleted",
  broken: "the skill's folder moved",
  foreign_conflict: "another app's file is there",
};

const STATUS_TONE: Record<SkillExposureStatus, StatusDotTone> = {
  healthy: "accent",
  missing: "danger",
  broken: "danger",
  foreign_conflict: "faint",
};

export function toSkillExposureView(exposure: SkillExposurePayload): SkillExposureView {
  const ours = exposure.status !== "foreign_conflict";
  return {
    target: exposure.target,
    path: exposure.path,
    status: exposure.status,
    sentence: STATUS_SENTENCE[exposure.status],
    tone: STATUS_TONE[exposure.status],
    repairable: exposure.status === "missing" || exposure.status === "broken",
    removable: ours,
    stale: exposure.status === "missing" || exposure.status === "broken",
  };
}

export function skillExposureViews(skill: SkillPayload): SkillExposureView[] {
  return (skill.exposures ?? []).map(toSkillExposureView);
}

/**
 * Bundled skills have no directory to link, and profile-owned skills are not
 * exposable in this release. Both render no expose affordance at all — absent,
 * not disabled, so the surface never offers a dead end.
 */
const INELIGIBLE_SOURCES = new Set(["bundled", "profile", "workspace_profile"]);

export function isSkillExposable(skill: SkillPayload): boolean {
  if (INELIGIBLE_SOURCES.has(skill.source)) return false;
  return typeof skill.dir === "string" && skill.dir.trim() !== "";
}

const RESULT_SENTENCE: Record<string, string> = {
  rolled_back: "completed, then undone",
  expose_name_conflict: "a file with this name is already there",
  expose_target_disabled: "that source is turned off",
  expose_target_invalid: "custom folders can't receive links",
  expose_link_unsupported: "this filesystem can't create the link",
  expose_foreign_link: "that link isn't ours to remove",
  skill_not_exposable: "this skill has no folder of its own to link",
  profile_skill_not_exposable: "profile skills can't be exposed yet",
  unsafe_skill_name: "this skill's name can't be used as a folder name",
};

export interface SkillExposeResultView {
  target: string;
  ok: boolean;
  /** Plain sentence for the outcome. */
  sentence: string;
  /** The daemon's code, kept verbatim. */
  code: string | null;
  rolledBack: boolean;
}

/**
 * Renders one line per target the operation touched. A target that succeeded
 * and was then compensated is reported as undone rather than quietly dropped.
 */
export function skillExposeResultViews(
  results: readonly SkillExposeResult[]
): SkillExposeResultView[] {
  return results.map(result => {
    const failure = result.error ?? result.cleanup_error ?? null;
    const code = failure?.code ?? null;
    const rolledBack = code === "rolled_back";
    const sentence =
      code === null
        ? "done"
        : (RESULT_SENTENCE[code] ?? failure?.message?.trim() ?? "the daemon refused this target");
    return { target: result.target, ok: result.ok, sentence, code, rolledBack };
  });
}
