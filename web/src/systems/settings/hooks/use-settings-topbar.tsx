import { useNavigate } from "@tanstack/react-router";

import { useTopbarSlot, type TopbarCrumb } from "@agh/ui";

import { SETTINGS_SECTIONS, settingsSectionPath } from "../lib/sections";
import type { SettingsSectionSlug } from "../types";

export interface UseSettingsTopbarOptions {
  status?: React.ReactNode;
  actions?: React.ReactNode;
}

/**
 * Publishes one settings section's identity into the window topbar as the
 * drill-in trail "Settings / <Section>" (design `.w2head` contract); the
 * parent crumb returns to the default section.
 */
export function useSettingsTopbar(
  slug: SettingsSectionSlug,
  { status, actions }: UseSettingsTopbarOptions = {}
) {
  const navigate = useNavigate();
  const section = SETTINGS_SECTIONS.find(entry => entry.slug === slug);

  const crumbs: readonly TopbarCrumb[] = [
    {
      id: "settings-root",
      label: "Settings",
      onSelect: () => {
        void navigate({ to: settingsSectionPath(SETTINGS_SECTIONS[0].slug) });
      },
    },
  ];

  useTopbarSlot({
    crumb: section?.label ?? slug,
    crumbs,
    status,
    actions,
  });
}
