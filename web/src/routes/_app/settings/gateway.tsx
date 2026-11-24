import { createFileRoute, redirect } from "@tanstack/react-router";

import { settingsSectionPath } from "@/systems/settings";

/**
 * Remote access (`/settings/gateway`) was replaced by Computers (`/settings/computers`)
 * (`_uiux.md` U1, U10). Old links and persisted windows land on Computers instead of 404ing.
 */
export const Route = createFileRoute("/_app/settings/gateway")({
  beforeLoad: redirectRetiredGatewaySection,
  component: RetiredGatewayRedirect,
});

function redirectRetiredGatewaySection() {
  throw redirect({ to: settingsSectionPath("computers"), replace: true });
}

function RetiredGatewayRedirect() {
  return null;
}
