// Suite: gateway exposure projection
// Invariant: what the operator is shown about reachability comes from verified runtime state —
// a provider claim that has not passed verification never reads as reachable, and local-only is a
// runtime fact rather than an intent.
// Owning layer: the gateway read model between GET /api/gateway/status and the operator surface.
import { describe, expect, it } from "vitest";

import { gatewayStatusFixture, gatewayTierFixture } from "../../mocks/fixtures";
import type { GatewayStatus } from "../../types";
import {
  buildGatewayExposureModel,
  isLocalOnly,
  tierReachability,
} from "../gateway-exposure-model";

function reachablePublicStatus(surfaces: GatewayStatus["surfaces"]): GatewayStatus {
  return gatewayStatusFixture({
    addresses: [{ address: "https://public.gateway.test", live: true, tier: "public" }],
    providers: [
      {
        cause: "",
        desired: "enabled",
        generation: 1,
        health: "healthy",
        name: "connectivity-fixture",
        observed: "up",
        tier: "public",
      },
    ],
    surfaces,
    tiers: [
      { advertised: false, desired: "disabled", observed: "down", tier: "private" },
      { advertised: true, desired: "enabled", observed: "up", tier: "public" },
    ],
  });
}

describe("gateway exposure model", () => {
  it("Should report local-only while nothing is advertised and no address is verified", () => {
    const status = gatewayStatusFixture();
    expect(isLocalOnly(status)).toBe(true);
    expect(buildGatewayExposureModel(status).localOnly).toBe(true);
  });

  it("Should read an unverified provider claim as establishing, never as reachable", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "establishing", verified: false })
    );
    expect(tierReachability(status, "private")).toBe("pending");
    expect(isLocalOnly(status)).toBe(true);
  });

  it("Should read a provider that reports up but has no verified address as establishing", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "up", verified: false })
    );
    expect(tierReachability(status, "private")).toBe("pending");
  });

  it("Should read a supervised provider failure as degraded", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({
        tier: "public",
        observed: "degraded",
        verified: false,
        cause: "provider handshake failed",
      })
    );
    expect(tierReachability(status, "public")).toBe("degraded");
  });

  it("Should read a fail-closed refusal as refused rather than as a pending tier", () => {
    const status = gatewayStatusFixture({
      ...gatewayTierFixture({ tier: "private", observed: "down", verified: false }),
      refusal: { cause: "authentication is inactive", fix: "pair a device first" },
    });
    expect(tierReachability(status, "private")).toBe("refused");
  });

  it("Should report reachable only once an advertised tier has a verified live address", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "up", verified: true })
    );
    expect(tierReachability(status, "private")).toBe("reachable");
    expect(isLocalOnly(status)).toBe(false);
  });

  it("Should never list an address the daemon has not marked live", () => {
    const status = gatewayStatusFixture({
      ...gatewayTierFixture({ tier: "private", observed: "up", verified: true }),
      addresses: [
        { address: "https://verified.gateway.test", live: true, tier: "private" },
        { address: "https://claimed.gateway.test", live: false, tier: "private" },
      ],
    });
    const row = buildGatewayExposureModel(status).rows.find(
      candidate => candidate.id === "private-operator-ui"
    );
    expect(row?.addresses.map(address => address.address)).toEqual([
      "https://verified.gateway.test",
    ]);
  });

  it("Should offer only the exposure decisions the runtime accepts", () => {
    const model = buildGatewayExposureModel(gatewayStatusFixture());
    expect(model.rows.map(row => `${row.tier}:${row.surface}`)).toEqual([
      "private:operator_ui",
      "public:webhook_ingress",
      "public:operator_ui",
    ]);
  });

  it("Should mark public operator access as the row that needs consent", () => {
    const model = buildGatewayExposureModel(gatewayStatusFixture());
    expect(model.rows.filter(row => row.requiresConsent).map(row => row.id)).toEqual([
      "public-operator-ui",
    ]);
  });

  it("Should carry the surface generation so a transition is fenced against a stale read", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "up", verified: true, generation: 7 })
    );
    const row = buildGatewayExposureModel(status).rows.find(
      candidate => candidate.id === "private-operator-ui"
    );
    expect(row?.generation).toBe(7);
  });

  it("Should surface the configuration ceiling separately from operator intent", () => {
    const model = buildGatewayExposureModel(gatewayStatusFixture({ enabled: false }));
    expect(model.ceilingEnabled).toBe(false);
  });

  it("Should not lend a served surface's reachability or address to the other surface on its tier", () => {
    // Public webhook ingress is verified and live; public operator access is
    // still off. The two share a tier but not a truth.
    const status = reachablePublicStatus([
      {
        desired: "enabled",
        generation: 2,
        observed: "on",
        surface: "webhook_ingress",
        tier: "public",
      },
      {
        desired: "disabled",
        generation: 5,
        observed: "off",
        surface: "operator_ui",
        tier: "public",
      },
    ]);
    const rows = buildGatewayExposureModel(status).rows;
    const ingress = rows.find(row => row.id === "public-webhook-ingress");
    const operator = rows.find(row => row.id === "public-operator-ui");

    expect(ingress?.reachability).toBe("reachable");
    expect(ingress?.addresses.map(address => address.address)).toEqual([
      "https://public.gateway.test",
    ]);

    expect(operator?.reachability).toBe("off");
    expect(operator?.addresses).toEqual([]);
  });

  it("Should hold a desired-but-unserved surface at pending even while its tier is reachable", () => {
    const status = reachablePublicStatus([
      {
        desired: "enabled",
        generation: 2,
        observed: "on",
        surface: "webhook_ingress",
        tier: "public",
      },
      // The operator asked for public operator access; reconciliation has not
      // served it yet.
      {
        desired: "enabled",
        generation: 6,
        observed: "off",
        surface: "operator_ui",
        tier: "public",
      },
    ]);
    const operator = buildGatewayExposureModel(status).rows.find(
      row => row.id === "public-operator-ui"
    );

    expect(operator?.reachability).toBe("pending");
    expect(operator?.addresses).toEqual([]);
  });

  it("Should keep a served surface degraded when its provider failed", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "degraded", verified: false, served: true })
    );
    const row = buildGatewayExposureModel(status).rows.find(
      candidate => candidate.id === "private-operator-ui"
    );

    expect(row?.reachability).toBe("degraded");
    expect(row?.addresses).toEqual([]);
  });

  it("Should keep a served surface refused when the safety policy refused the tier", () => {
    const status = gatewayStatusFixture({
      ...gatewayTierFixture({ tier: "private", observed: "down", verified: false, served: true }),
      refusal: { cause: "authentication is inactive", fix: "pair a device first" },
    });
    const row = buildGatewayExposureModel(status).rows.find(
      candidate => candidate.id === "private-operator-ui"
    );

    expect(row?.reachability).toBe("refused");
  });
});
