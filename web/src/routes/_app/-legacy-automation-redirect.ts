import { redirect } from "@tanstack/react-router";

import { redirectLegacyAutomationURL } from "@/systems/automation";

/**
 * Replace-navigates a legacy `/jobs*` or `/triggers*` URL to `/automations*`.
 * Shim: the `jobs*` / `triggers*` route stubs and this helper are removed in v0.5.0.
 */
export function redirectLegacyAutomationRoute(location: {
  pathname: string;
  search: Record<string, unknown>;
}): never {
  const target = redirectLegacyAutomationURL(location.pathname, location.search);
  const detail = target ? /^\/automations\/(jobs|triggers)\/([^/]+)$/.exec(target.pathname) : null;
  if (detail?.[1] === "jobs") {
    throw redirect({
      to: "/automations/jobs/$jobId",
      params: { jobId: decodeURIComponent(detail[2]) },
      replace: true,
    });
  }
  if (detail?.[1] === "triggers") {
    throw redirect({
      to: "/automations/triggers/$triggerId",
      params: { triggerId: decodeURIComponent(detail[2]) },
      replace: true,
    });
  }
  throw redirect({ to: "/automations", search: target?.search ?? {}, replace: true });
}
