import { redirectLegacyAutomationURL } from "@/systems/automation";

import { parseWindowManagerLayoutDocument } from "./window-manager-layout-schema";
import type { WindowManagerLayoutDocument } from "./window-manager-layout-types";

/** App ids retired by the Automations merge; stored version-4 documents may still name them. */
const RETIRED_LAYOUT_APPS = new Set(["jobs", "triggers"]);

type WireRecord = Record<string, unknown>;

function isRecord(value: unknown): value is WireRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function upgradeRoute(route: unknown): unknown {
  if (!isRecord(route) || typeof route.pathname !== "string") return route;
  const search = isRecord(route.search) ? route.search : {};
  const redirect = redirectLegacyAutomationURL(route.pathname, search);
  return redirect ? { ...route, pathname: redirect.pathname, search: redirect.search } : route;
}

function upgradeWindow(window: unknown): unknown {
  if (!isRecord(window) || typeof window.app !== "string" || !RETIRED_LAYOUT_APPS.has(window.app)) {
    return window;
  }
  return {
    ...window,
    app: "automations",
    route: upgradeRoute(window.route),
    nav_stack: Array.isArray(window.nav_stack)
      ? window.nav_stack.map(upgradeRoute)
      : window.nav_stack,
  };
}

/**
 * Version-4 exports made before the Automations merge keep importing: the
 * same permanent upgrade the daemon applies (ADR-002, `MigrateSnapshotV4`)
 * moves their Jobs and Triggers windows to Automations on the rewritten
 * routes. No window is dropped.
 */
export function upgradeLayoutDocumentWire(value: unknown): unknown {
  if (!isRecord(value) || value.version !== 4) return value;
  const windows = isRecord(value.windows)
    ? Object.fromEntries(Object.entries(value.windows).map(([id, w]) => [id, upgradeWindow(w)]))
    : value.windows;
  return { ...value, version: 5, windows };
}

/** Parses a layout file the operator imports, accepting version 4 and 5 documents. */
export function parseImportedWindowManagerLayoutDocument(
  value: unknown
): WindowManagerLayoutDocument {
  return parseWindowManagerLayoutDocument(upgradeLayoutDocumentWire(value));
}
