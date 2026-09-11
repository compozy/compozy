// Suite: gateway capability store
// Invariant: capabilities derive from the latched tier alone and re-evaluate on every tier
// observation without an app restart (BR-4). A loopback 403 re-evaluates from the refusing
// response (stale-map convergence, US-002.EC-2) and records the backstop signal; unrelated 403
// codes never classify, and a `local` latch clears a stale loopback refusal.
// Owning layer: the app-wide capability state between the shared transport and the gating consumers.
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { reportGatewayListenerTier, reportGatewayResponse } from "@/lib/gateway-access-signal";

import { capabilitiesForTier } from "../../lib/gateway-capabilities";
import {
  gatewayCapabilityStore,
  startGatewayCapabilityObserver,
} from "../gateway-capability-store";

function jsonResponse(status: number, payload: unknown, tier?: string): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json",
      ...(tier ? { "X-Compozy-Gateway-Tier": tier } : {}),
    },
  });
}

describe("gateway capability store", () => {
  let stop: () => void;

  beforeEach(() => {
    gatewayCapabilityStore.trigger.capabilitiesReset();
    stop = startGatewayCapabilityObserver();
  });

  afterEach(() => {
    stop();
    gatewayCapabilityStore.trigger.capabilitiesReset();
  });

  it("Should start unlatched and default-hidden", () => {
    const { context } = gatewayCapabilityStore.getSnapshot();
    expect(context.tier).toBeUndefined();
    expect(context.capabilities).toEqual(capabilitiesForTier(undefined));
  });

  it("Should re-evaluate capabilities when the tier latches or changes, without a restart", () => {
    // UT-007 (state, tier side): a new `/api/status` observation moves the
    // capability set with no app restart (US-004.AC-1).
    reportGatewayListenerTier("private");
    expect(gatewayCapabilityStore.getSnapshot().context.capabilities).toEqual(
      capabilitiesForTier("private")
    );

    reportGatewayListenerTier("local");
    expect(gatewayCapabilityStore.getSnapshot().context.capabilities).toEqual(
      capabilitiesForTier("local")
    );
  });

  it("Should re-evaluate from a loopback 403 response", async () => {
    // UT-007 (state, response side): one refusing response latches its tier
    // header and records the loopback signal — a stale map converges from the
    // response itself (US-002.EC-2).
    await reportGatewayResponse(
      jsonResponse(403, { error: "forbidden", code: "loopback_mutation_required" }, "private")
    );

    expect(gatewayCapabilityStore.getSnapshot().context).toMatchObject({
      tier: "private",
      capabilities: capabilitiesForTier("private"),
      loopbackOnly: "mutation",
    });
  });

  it("Should record the loopback backstop without degrading a latched local tier", async () => {
    // A non-loopback local bind (e.g. LAN address) latches `local` yet still
    // refuses privileged mutations: the tier map stays truthful and the
    // backstop records exactly what the daemon refused.
    reportGatewayListenerTier("local");

    await reportGatewayResponse(jsonResponse(403, { code: "loopback_api_required" }));

    expect(gatewayCapabilityStore.getSnapshot().context).toMatchObject({
      tier: "local",
      capabilities: capabilitiesForTier("local"),
      loopbackOnly: "api",
    });
  });

  it("Should not record the loopback backstop for an unrelated 403 code", async () => {
    await reportGatewayResponse(
      jsonResponse(403, { error: "forbidden", code: "workspace_home_forbidden" }, "private")
    );

    expect(gatewayCapabilityStore.getSnapshot().context.loopbackOnly).toBeUndefined();
  });

  it("Should keep the loopback backstop across remote tiers and clear it on a local latch", async () => {
    await reportGatewayResponse(
      jsonResponse(403, { code: "loopback_mutation_required" }, "private")
    );
    reportGatewayListenerTier("public");
    expect(gatewayCapabilityStore.getSnapshot().context.loopbackOnly).toBe("mutation");

    // On a loopback-bound listener the refused call would succeed, so a
    // `local` latch makes the recorded refusal stale.
    reportGatewayListenerTier("local");
    expect(gatewayCapabilityStore.getSnapshot().context.loopbackOnly).toBeUndefined();
  });

  it("Should keep exactly one subscription when started more than once", async () => {
    const second = startGatewayCapabilityObserver();
    second();

    await reportGatewayResponse(
      jsonResponse(403, { code: "loopback_mutation_required" }, "private")
    );
    expect(gatewayCapabilityStore.getSnapshot().context.loopbackOnly).toBe("mutation");
  });
});
