// Suite: session detail polling policy
// Invariant: durable starting sessions converge quickly while terminal sessions stop polling.
// Boundary IN: TanStack session detail query options.
// Boundary OUT: HTTP transport and lifecycle persistence, owned by their adapter/runtime suites.
import { describe, expect, it } from "vitest";

import type { SessionInteractionRecord, SessionPayload } from "../../types";
import {
  sessionAcrossProfilesOptions,
  sessionCommandsOptions,
  sessionDetailOptions,
  sessionResolvedInteractionsOptions,
  sessionScopedDetailOptions,
} from "../query-options";

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

describe("sessionCommandsOptions", () => {
  it("Should key the command catalog by workspace and session", () => {
    const options = sessionCommandsOptions(" ws_alpha ", "sess_1");
    expect(options.queryKey).toEqual(["session-commands", "ws_alpha", "sess_1"]);
    expect(options.enabled).toBe(true);
    expect(sessionCommandsOptions("", "sess_1").enabled).toBe(false);
    expect(sessionCommandsOptions("   ", "sess_1").enabled).toBe(false);
    expect(sessionCommandsOptions("ws_alpha", "sess_1", false).enabled).toBe(false);
  });
});

describe("sessionScopedDetailOptions", () => {
  it("Should key the profile-enforced read by the lens so two profiles never share an entry", () => {
    expect(sessionScopedDetailOptions("sess_1", { profile: "marketing" }).queryKey).toEqual([
      "sessions",
      "by-id",
      "sess_1",
      "marketing",
    ]);
    expect(sessionScopedDetailOptions("sess_1", { profile: "marketing" }).queryKey).not.toEqual(
      sessionScopedDetailOptions("sess_1", { profile: "default" }).queryKey
    );
  });

  it("Should keep the id above the lens so one mutation can sweep every lens holding it", () => {
    const scoped = sessionScopedDetailOptions("sess_1", { profile: "marketing" }).queryKey;
    const aggregate = sessionAcrossProfilesOptions("sess_1").queryKey;
    // Both live under the same by-id prefix; a mutation knows the session, never
    // which lens some other surface is looking through.
    expect(scoped.slice(0, 3)).toEqual(["sessions", "by-id", "sess_1"]);
    expect(aggregate.slice(0, 3)).toEqual(["sessions", "by-id", "sess_1"]);
    expect(scoped).not.toEqual(aggregate);
  });

  it("Should give the aggregate the reserved identity rather than the default profile's", () => {
    expect(sessionAcrossProfilesOptions("sess_1").queryKey).not.toEqual(
      sessionScopedDetailOptions("sess_1", { profile: "default" }).queryKey
    );
  });

  it("Should refuse to run without a lens, because an absent selector resolves default", () => {
    expect(sessionScopedDetailOptions("sess_1", { profile: "" }).enabled).toBe(false);
    expect(sessionScopedDetailOptions("", { profile: "marketing" }).enabled).toBe(false);
  });

  it("Should not retry, so a 404 stays a boundary decision rather than a delay", () => {
    expect(sessionScopedDetailOptions("sess_1", { profile: "marketing" }).retry).toBe(false);
  });

  it("Should stop polling entirely when the window does not own the live tail", () => {
    const refetchInterval = sessionScopedDetailOptions(
      "sess_1",
      { profile: "marketing" },
      {
        liveTail: false,
      }
    ).refetchInterval;
    if (typeof refetchInterval !== "function") throw new Error("expected a state-aware interval");
    expect(refetchInterval({ state: { data: { state: "active" } } } as never)).toBe(false);
  });
});

describe("sessionResolvedInteractionsOptions", () => {
  const previousRows: SessionInteractionRecord[] = [
    {
      interaction_id: "int-1",
      kind: "permission",
      provider_request_id: "turn_001:perm_1",
      status: "resolved",
      created_at: "2026-09-05T10:00:00Z",
      resolution: "reject-once",
      resolved_by: "operator",
    },
  ];

  function placeholderFrom(
    options: ReturnType<typeof sessionResolvedInteractionsOptions>,
    previousKey: readonly unknown[]
  ) {
    const placeholderData = options.placeholderData;
    if (typeof placeholderData !== "function") throw new Error("expected a scoped placeholder");
    return placeholderData(previousRows, { queryKey: previousKey } as never);
  }

  it("Should fence the read by the decided request ids inside the session's interactions scope", () => {
    const options = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["b", "a"]),
    });
    expect(options.queryKey).toEqual([
      "sessions",
      "workspace",
      "ws_alpha",
      "detail",
      "sess_1",
      "interactions",
      "resolved",
      ["a", "b"],
    ]);
    expect(sessionResolvedInteractionsOptions("", "sess_1").enabled).toBe(false);
    expect(
      sessionResolvedInteractionsOptions("ws_alpha", "sess_1", { enabled: false }).enabled
    ).toBe(false);
  });

  it("Should keep previous rows only across a fence change in the same workspace and session", () => {
    const previous = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });
    const next = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1", "turn_001:perm_2"]),
    });

    expect(placeholderFrom(next, previous.queryKey)).toBe(previousRows);
  });

  it("Should never present another session's or workspace's rows while the new read is in flight", () => {
    const previous = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });
    const otherSession = sessionResolvedInteractionsOptions("ws_alpha", "sess_2", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });
    const otherWorkspace = sessionResolvedInteractionsOptions("ws_beta", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });

    expect(placeholderFrom(otherSession, previous.queryKey)).toBeUndefined();
    expect(placeholderFrom(otherWorkspace, previous.queryKey)).toBeUndefined();
    expect(placeholderFrom(otherSession, [])).toBeUndefined();
  });
});
