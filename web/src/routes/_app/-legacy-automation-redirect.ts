import { redirect } from "@tanstack/react-router";

import { parseAutomationDetailPath, redirectLegacyAutomationURL } from "@/systems/automation";

/**
 * Replace-navigates a legacy `/jobs*` or `/triggers*` URL to `/automations*`.
 * Shim: the `jobs*` / `triggers*` route stubs and this helper are removed in v0.5.0.
 */
export function redirectLegacyAutomationRoute(location: {
  pathname: string;
  search: Record<string, unknown>;
}): never {
  const target = redirectLegacyAutomationURL(location.pathname, location.search);
  const detail = target ? parseAutomationDetailPath(target.pathname) : null;
  if (detail?.kind === "jobs") {
    throw redirect({
      to: "/automations/jobs/$jobId",
      params: { jobId: detail.id },
      replace: true,
    });
  }
  if (detail?.kind === "triggers") {
    throw redirect({
      to: "/automations/triggers/$triggerId",
      params: { triggerId: detail.id },
      replace: true,
    });
  }
  throw redirect({ to: "/automations", search: target?.search ?? {}, replace: true });
}
