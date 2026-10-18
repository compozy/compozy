import { createFileRoute } from "@tanstack/react-router";

import { redirectLegacyAutomationRoute } from "./-legacy-automation-redirect";

// Shim: legacy URL redirect stub; remove in v0.5.0.
export const Route = createFileRoute("/_app/triggers")({
  validateSearch: (search: Record<string, unknown>) => search,
  beforeLoad: ({ location, search }) =>
    redirectLegacyAutomationRoute({ pathname: location.pathname, search }),
});
