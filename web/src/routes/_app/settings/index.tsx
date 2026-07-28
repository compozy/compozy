import { createFileRoute, redirect } from "@tanstack/react-router";

import { SETTINGS_SECTIONS, settingsSectionPath } from "@/systems/settings";

export const Route = createFileRoute("/_app/settings/")({
  beforeLoad: redirectToDefaultSettingsSection,
  component: SettingsIndexRedirect,
});

function redirectToDefaultSettingsSection() {
  throw redirect({ to: settingsSectionPath(SETTINGS_SECTIONS[0].slug) });
}

function SettingsIndexRedirect() {
  return null;
}
