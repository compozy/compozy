// Suite: Automations URL contract
// Invariant: listing search normalizes unknown values away and legacy URLs map to /automations.
// Boundary IN: raw router search records and legacy pathnames.
// Boundary OUT: router navigation (route stubs own the replace).
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { redirectLegacyAutomationURL } from "../automation-redirects";
import {
  automationRouteHasActiveFilters,
  validateAutomationsSearch,
} from "../automation-route-search";

describe("validateAutomationsSearch", () => {
  it("Should drop unknown start, view and target values", () => {
    expect(validateAutomationsSearch({ start: "bogus", view: "grid", target: "x" })).toEqual({});
  });

  it("Should keep every known listing param", () => {
    expect(
      validateAutomationsSearch({
        start: "event",
        q: " digest ",
        enabled: "true",
        scope: "workspace",
        source: "config",
        target: "loop",
        loop: "software-delivery",
        view: "cards",
      })
    ).toEqual({
      start: "event",
      q: "digest",
      enabled: true,
      scope: "workspace",
      source: "config",
      target: "loop",
      loop: "software-delivery",
      view: "cards",
    });
  });

  it("Should accept a webhook start only as an editor preselection", () => {
    expect(validateAutomationsSearch({ start: "webhook" })).toEqual({});
    expect(validateAutomationsSearch({ create: "1", start: "webhook" })).toEqual({
      create: "1",
      start: "webhook",
    });
  });

  it("Should treat a Start view as an active filter but not a Loop create seed", () => {
    expect(automationRouteHasActiveFilters({ start: "schedule" })).toBe(true);
    expect(automationRouteHasActiveFilters({ create: "loop", loop: "software-delivery" })).toBe(
      false
    );
    expect(automationRouteHasActiveFilters({})).toBe(false);
  });
});

interface RetiredAppRouteVector {
  app: "jobs" | "triggers";
  input: { pathname: string; search: Record<string, unknown> };
  expected: { pathname: string; search: Record<string, unknown> };
}

/** The daemon's `RewriteRetiredAppRoute` vectors (UT-126); both sides must agree on each. */
const retiredAppRouteVectors = JSON.parse(
  readFileSync(
    join(__dirname, "../../../../../../internal/windowmanager/testdata/retired_app_routes.json"),
    "utf8"
  )
) as RetiredAppRouteVector[];

describe("redirectLegacyAutomationURL", () => {
  it("Should load the shared daemon vectors", () => {
    expect(retiredAppRouteVectors.length).toBeGreaterThan(0);
  });

  it.each(retiredAppRouteVectors.map(vector => [vector.input.pathname, vector] as const))(
    "Should match the daemon rewrite for %s",
    (_pathname, vector) => {
      expect(redirectLegacyAutomationURL(vector.input.pathname, vector.input.search)).toEqual(
        vector.expected
      );
    }
  );

  it.each([
    [
      "/jobs",
      { enabled: true, q: "nightly" },
      { pathname: "/automations", search: { start: "schedule", enabled: true, q: "nightly" } },
    ],
    ["/jobs/morning-digest", {}, { pathname: "/automations/jobs/morning-digest", search: {} }],
    [
      "/triggers",
      { event: "session.stopped", source: "dynamic" },
      {
        pathname: "/automations",
        search: { start: "event", q: "session.stopped", source: "dynamic" },
      },
    ],
    [
      "/triggers/rerun-delivery",
      {},
      { pathname: "/automations/triggers/rerun-delivery", search: {} },
    ],
    [
      "/jobs",
      { create: "loop", loop: "software-delivery" },
      {
        pathname: "/automations",
        search: { create: "loop", start: "schedule", loop: "software-delivery" },
      },
    ],
    [
      "/triggers",
      { create: "loop", loop: "software-delivery" },
      {
        pathname: "/automations",
        search: { create: "loop", start: "event", loop: "software-delivery" },
      },
    ],
    [
      "/jobs",
      { scope: "all", view: "rows" },
      { pathname: "/automations", search: { start: "schedule" } },
    ],
  ])("Should map %s %o", (pathname, search, expected) => {
    expect(redirectLegacyAutomationURL(pathname, search)).toEqual(expected);
  });

  it("Should ignore paths outside the legacy routes", () => {
    expect(redirectLegacyAutomationURL("/automations", {})).toBeNull();
    expect(redirectLegacyAutomationURL("/jobsite", {})).toBeNull();
  });
});
