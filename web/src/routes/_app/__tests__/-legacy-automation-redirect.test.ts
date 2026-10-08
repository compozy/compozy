// Suite: Legacy automation route stubs (shim, removed in v0.5.0)
// Invariant: /jobs* and /triggers* replace-navigate to /automations* without a history entry.
// Boundary IN: the stub routes' beforeLoad location.
// Boundary OUT: the URL table itself (automation-route-search suite).
import { isRedirect } from "@tanstack/react-router";
import { describe, expect, it } from "vitest";

import { Route as JobsRoute } from "../jobs";
import { Route as TriggersRoute } from "../triggers";

type BeforeLoad = (args: {
  location: { pathname: string };
  search: Record<string, unknown>;
}) => unknown;

function redirectOf(route: { options: { beforeLoad?: unknown } }, pathname: string, search = {}) {
  try {
    (route.options.beforeLoad as BeforeLoad)({ location: { pathname }, search });
  } catch (error) {
    if (isRedirect(error)) return error.options;
    throw error;
  }
  throw new Error(`Expected ${pathname} to redirect`);
}

describe("legacy automation route stubs", () => {
  it("Should replace a job detail URL with its automation detail", () => {
    expect(redirectOf(JobsRoute, "/jobs/morning-digest")).toMatchObject({
      to: "/automations/jobs/$jobId",
      params: { jobId: "morning-digest" },
      replace: true,
    });
  });

  it("Should replace the triggers list with the On events view, event as the query", () => {
    expect(redirectOf(TriggersRoute, "/triggers", { event: "session.stopped" })).toMatchObject({
      to: "/automations",
      search: { start: "event", q: "session.stopped" },
      replace: true,
    });
    expect(redirectOf(TriggersRoute, "/triggers/rerun-delivery")).toMatchObject({
      to: "/automations/triggers/$triggerId",
      params: { triggerId: "rerun-delivery" },
      replace: true,
    });
  });
});
