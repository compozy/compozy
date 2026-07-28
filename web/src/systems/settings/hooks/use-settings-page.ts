import { useMatchRoute } from "@tanstack/react-router";

import {
  SETTINGS_ROOT_PATH,
  SETTINGS_SECTIONS,
  settingsSectionPath,
  useSettingsRestart,
  type SettingsSectionDescriptor,
} from "@/systems/settings";

interface UseSettingsPageOptions {
  currentSlug?: string;
}

function useSettingsPage(options: UseSettingsPageOptions = {}) {
  const matchRoute = useMatchRoute();
  const restart = useSettingsRestart();

  let matchedSection: SettingsSectionDescriptor | null = null;
  if (options.currentSlug) {
    matchedSection =
      SETTINGS_SECTIONS.find(section => section.slug === options.currentSlug) ?? null;
  }
  if (!matchedSection) {
    matchedSection =
      SETTINGS_SECTIONS.find(section =>
        matchRoute({ to: settingsSectionPath(section.slug), fuzzy: true })
      ) ?? null;
  }

  const isRestartNoticeVisible =
    (restart.isRestartRequired && !restart.isNoticeSnoozed) ||
    restart.isPolling ||
    restart.isSuccessful ||
    restart.isFailed;

  const restartNotice = {
    isVisible: isRestartNoticeVisible,
    isRestartRequired: restart.isRestartRequired,
    isPolling: restart.isPolling,
    isSuccessful: restart.isSuccessful,
    isFailed: restart.isFailed,
    operationId: restart.operationId,
    status: restart.status,
    failureReason: restart.failureReason,
    activeSessionCount: restart.activeSessionCount,
    lastMutation: restart.lastMutation,
    trigger: restart.trigger,
    isTriggerPending: restart.isTriggerPending,
    triggerError: restart.triggerError,
    dismiss: restart.dismiss,
  } as const;

  return {
    sections: SETTINGS_SECTIONS,
    rootPath: SETTINGS_ROOT_PATH,
    activeSection: matchedSection,
    activeSectionSlug: matchedSection?.slug ?? null,
    sectionPath: (slug: SettingsSectionDescriptor["slug"]) => settingsSectionPath(slug),
    restart: restartNotice,
  };
}

export { useSettingsPage };
