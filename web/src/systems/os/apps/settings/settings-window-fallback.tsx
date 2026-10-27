import { cn, Skeleton, SkeletonText } from "@compozy/ui";

import type { OsWindowRoute } from "../../lib/os-types";
import { SETTINGS_NAV_ITEM_CLASS, settingsNavItemTone } from "./settings-nav-classes";
import { SettingsNavFrame, SettingsNavGroup, SettingsNavItemContent } from "./settings-nav-parts";
import {
  SETTINGS_SECTION_GROUPS,
  SETTINGS_SECTIONS,
  SettingsPageState,
  settingsSectionSlugFromPathname,
  type SettingsSectionSlug,
} from "@/systems/settings";

/**
 * The section list as the nav shows it, without the controls: the same frame, the real section
 * names and the open section selected, so the rail is final from the first frame. The search
 * field and the daemon status line, which need the live shell, are blocks. It is not the
 * navigation: its items are inert, so it keeps its own test id and never answers for the live nav.
 */
function SettingsNavSkeleton({ activeSlug }: { activeSlug: SettingsSectionSlug }) {
  return (
    <SettingsNavFrame
      aria-hidden="true"
      data-testid="settings-section-nav-skeleton"
      footer={<SkeletonText className="w-28 text-eyebrow" />}
      search={<Skeleton className="mt-2.5 h-search w-full rounded-md" />}
    >
      {SETTINGS_SECTION_GROUPS.map(group => {
        const sections = SETTINGS_SECTIONS.filter(section => section.group === group.id);
        return (
          <SettingsNavGroup key={group.id} label={group.label}>
            {sections.map(section => {
              const isActive = section.slug === activeSlug;
              return (
                <div
                  className={cn(SETTINGS_NAV_ITEM_CLASS, settingsNavItemTone(isActive))}
                  key={section.slug}
                >
                  <SettingsNavItemContent isActive={isActive} section={section} />
                </div>
              );
            })}
          </SettingsNavGroup>
        );
      })}
    </SettingsNavFrame>
  );
}

/**
 * The settings window's body while its code loads: the window's own frame (the container that
 * switches between a strip and a rail, the nav and the section area) with the routed section's
 * loading shape in the area, exactly what the window shows while that section's page loads.
 */
export function SettingsWindowFallback({ route }: { route: OsWindowRoute }) {
  const slug = settingsSectionSlugFromPathname(route.pathname);
  return (
    <div className="@container flex min-h-0 flex-1 flex-col overflow-hidden">
      <div
        className="flex min-h-0 flex-1 flex-col @min-settings-takeover:flex-row"
        data-testid="settings-shell"
      >
        <SettingsNavSkeleton activeSlug={slug} />
        <div
          className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
          data-testid="settings-shell-outlet"
        >
          <SettingsPageState slug={slug} state="loading" />
        </div>
      </div>
    </div>
  );
}
