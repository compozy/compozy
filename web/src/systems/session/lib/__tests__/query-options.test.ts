// Suite: session detail polling policy
// Invariant: durable starting sessions converge quickly while terminal sessions stop polling.
// Boundary IN: TanStack session detail query options.
// Boundary OUT: HTTP transport and lifecycle persistence, owned by their adapter/runtime suites.
import { describe, expect, it } from "vitest";

import type { SessionPayload } from "../../types";
import { sessionDetailOptions } from "../query-options";

function pollingIntervalFor(state: SessionPayload["state"] | undefined): number | false {
  const refetchInterval = sessionDetailOptions("ws_alpha", "sess_1").refetchInterval;
  if (typeof refetchInterval !== "function") {
    throw new Error("Session detail polling must be state-aware");
  }

  const interval = refetchInterval({
    state: { data: state ? ({ state } as SessionPayload) : undefined },
  } as never);
  if (interval === undefined) {
    throw new Error("Session detail polling returned no interval");
  }
  return interval;
}

describe("sessionDetailOptions", () => {
  it("Should poll starting sessions rapidly until startup reaches a durable result", () => {
    expect(pollingIntervalFor("starting")).toBe(500);
    expect(pollingIntervalFor("active")).toBe(5_000);
    expect(pollingIntervalFor("stopping")).toBe(5_000);
    expect(pollingIntervalFor("stopped")).toBe(false);
    expect(pollingIntervalFor(undefined)).toBe(false);
  });
});
