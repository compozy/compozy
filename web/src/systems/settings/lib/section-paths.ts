import type { SettingsSectionSlug } from "../types";

export const SETTINGS_ROOT_PATH = "/settings" as const;
export const DEFAULT_SETTINGS_SECTION_SLUG = "general" as const satisfies SettingsSectionSlug;

export function settingsSectionPath(slug: SettingsSectionSlug): string {
  return `${SETTINGS_ROOT_PATH}/${slug}`;
}

/**
 * Retired section slugs and the section that absorbed each one. Bookmarks,
 * window routes and persisted layouts that still name a retired slug resolve
 * through this table instead of falling through to an unknown section.
 * Remove in v0.5.0.
 */
const RETIRED_SETTINGS_SECTION_SLUGS: Readonly<Record<string, SettingsSectionSlug>> = {
  defaults: "general",
};

/** The live section a retired slug now lives in, or null for any other slug. */
export function retiredSettingsSectionReplacement(slug: string): SettingsSectionSlug | null {
  return Object.hasOwn(RETIRED_SETTINGS_SECTION_SLUGS, slug)
    ? RETIRED_SETTINGS_SECTION_SLUGS[slug]
    : null;
}
