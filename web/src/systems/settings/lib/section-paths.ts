import type { SettingsSectionSlug } from "../types";

export const SETTINGS_ROOT_PATH = "/settings" as const;
export const DEFAULT_SETTINGS_SECTION_SLUG = "general" as const satisfies SettingsSectionSlug;

export function settingsSectionPath(slug: SettingsSectionSlug): string {
  return `${SETTINGS_ROOT_PATH}/${slug}`;
}
