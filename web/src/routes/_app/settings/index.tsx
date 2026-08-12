import { createFileRoute, redirect } from "@tanstack/react-router";

import { DEFAULT_SETTINGS_SECTION_SLUG, settingsSectionPath } from "@/systems/settings";

export const Route = createFileRoute("/_app/settings/")({
  beforeLoad: redirectToDefaultSettingsSection,
  component: SettingsIndexRedirect,
});

function redirectToDefaultSettingsSection() {
  throw redirect({ to: settingsSectionPath(DEFAULT_SETTINGS_SECTION_SLUG) });
}

function SettingsIndexRedirect() {
  return null;
}
