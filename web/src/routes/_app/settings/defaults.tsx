import { createFileRoute, redirect } from "@tanstack/react-router";

import { settingsSectionPath } from "@/systems/settings";

/**
 * Session defaults moved to General (#814). The retired route stays so old
 * links, window routes and persisted layouts land there instead of 404ing.
 * Remove in v0.5.0 with the retired-slug table.
 */
export const Route = createFileRoute("/_app/settings/defaults")({
  beforeLoad: redirectRetiredDefaultsSection,
  component: RetiredDefaultsRedirect,
});

function redirectRetiredDefaultsSection() {
  throw redirect({ to: settingsSectionPath("general"), replace: true });
}

function RetiredDefaultsRedirect() {
  return null;
}
