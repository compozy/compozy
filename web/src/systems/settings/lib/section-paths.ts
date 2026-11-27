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
  gateway: "computers",
};

/** The live section a retired slug now lives in, or null for any other slug. */
export function retiredSettingsSectionReplacement(slug: string): SettingsSectionSlug | null {
  return Object.hasOwn(RETIRED_SETTINGS_SECTION_SLUGS, slug)
    ? RETIRED_SETTINGS_SECTION_SLUGS[slug]
    : null;
}

interface SettingsRouteLike {
  pathname: string;
  search: Record<string, unknown>;
}

/**
 * The shortcut table moved from Layouts to Shortcuts. A route that still asks Layouts to land on a
 * command (`/settings/layouts?command=<id>` from an old link, a persisted window or an agent) now
 * belongs to Shortcuts; plain `/settings/layouts` stays Layouts. Returns the Shortcuts route, or
 * null when the route needs no change. Remove in v0.9.0.
 */
export function settingsRouteWithMovedShortcutCommand<Route extends SettingsRouteLike>(
  route: Route
): Route | null {
  const command = route.search.command;
  if (typeof command !== "string" || command.trim() === "") return null;
  if (route.pathname.replace(/\/+$/, "") !== settingsSectionPath("layouts")) return null;
  return { ...route, pathname: settingsSectionPath("shortcuts") };
}
