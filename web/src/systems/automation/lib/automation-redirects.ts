/**
 * Legacy `/jobs*` and `/triggers*` web URLs → `/automations*`.
 *
 * Shared by the redirect route stubs and their tests; the daemon's
 * `RewriteRetiredAppRoute` (window-manager snapshot v5 + command aliases) uses
 * the same vectors (`internal/windowmanager/testdata/retired_app_routes.json`).
 * Shim: the redirect stubs that call this are removed in v0.5.0.
 */

import { normalizeListingSearchValue } from "@/lib/listing-search";

import { validateAutomationsSearch, type AutomationsRouteSearch } from "./automation-route-search";

export interface LegacyAutomationRedirect {
  pathname: string;
  search: AutomationsRouteSearch;
}

const LEGACY_DETAIL = /^\/(jobs|triggers)\/([^/]+)\/?$/;
const LEGACY_LIST = /^\/(jobs|triggers)\/?$/;

export function redirectLegacyAutomationURL(
  pathname: string,
  search: Record<string, unknown>
): LegacyAutomationRedirect | null {
  const detail = LEGACY_DETAIL.exec(pathname);
  if (detail) {
    return { pathname: `/automations/${detail[1]}/${detail[2]}`, search: {} };
  }
  const list = LEGACY_LIST.exec(pathname);
  if (!list) return null;
  const start = list[1] === "jobs" ? "schedule" : "event";
  const { event, start: _ignoredStart, ...rest } = search;
  const query = normalizeListingSearchValue(rest.q) ?? normalizeListingSearchValue(event);
  return {
    pathname: "/automations",
    search: validateAutomationsSearch({ ...rest, q: query, start }),
  };
}

const AUTOMATION_DETAIL_PATH = /^\/automations\/(jobs|triggers)\/([^/]+)$/;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** `/automations/{jobs|triggers}/:id` → the daemon entity route and its decoded id. */
export function parseAutomationDetailPath(
  pathname: string
): { kind: "jobs" | "triggers"; id: string } | null {
  const match = AUTOMATION_DETAIL_PATH.exec(pathname);
  if (!match) return null;
  return { kind: match[1] as "jobs" | "triggers", id: decodePathSegment(match[2]) };
}
