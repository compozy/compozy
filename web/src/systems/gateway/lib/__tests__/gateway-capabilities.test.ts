// Suite: gateway capability model
// Invariant: the capability set mirrors the backend SurfaceSet route matrices — every flag on
// for the local tier, every flag off for both remote tiers, and the all-false set while the tier
// is unlatched (default-hidden, BR-2), so the UI never offers an affordance the tier cannot run.
// Owning layer: the gateway system's tier→capability mapping.
import { describe, expect, it } from "vitest";

import { capabilitiesForTier } from "../gateway-capabilities";

describe("capabilitiesForTier", () => {
  it("Should enable every capability on the local tier", () => {
    // UT-001: the local surface set registers every operator route group.
    expect(capabilitiesForTier("local")).toEqual({
      localTaskLifecycle: true,
      privilegedMutations: true,
      profileEnablementWrites: true,
      agentKernel: true,
      fullResourceRoutes: true,
    });
  });

  it("Should disable every capability on the private tier", () => {
    // UT-002: the private tier registers the remote read subset only.
    expect(capabilitiesForTier("private")).toEqual({
      localTaskLifecycle: false,
      privilegedMutations: false,
      profileEnablementWrites: false,
      agentKernel: false,
      fullResourceRoutes: false,
    });
  });

  it("Should disable every capability on the public tier", () => {
    // UT-003: the public operator matrix differs from private only in gateway
    // management scope, which the tier-aware gateway settings page owns — not
    // a capability flag here.
    expect(capabilitiesForTier("public")).toEqual(capabilitiesForTier("private"));
  });

  it("Should default to all-hidden while the tier is unlatched", () => {
    // UT-004: loopback-only affordances stay hidden until `/api/status` proves
    // `local` (BR-2) — no flash-then-hide on first paint.
    expect(capabilitiesForTier(undefined)).toEqual({
      localTaskLifecycle: false,
      privilegedMutations: false,
      profileEnablementWrites: false,
      agentKernel: false,
      fullResourceRoutes: false,
    });
  });
});
