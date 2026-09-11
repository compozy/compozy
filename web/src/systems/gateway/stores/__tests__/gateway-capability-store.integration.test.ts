// Suite: transport → capability store re-evaluation (IT-003, US-002.AC-1/AC-2, US-004.AC-1)
// Invariant: the real api-client response middleware hands a 403 loopback
// envelope to the gateway system, and the capability store re-evaluates from
// the refusing response — the backstop signal records, the capability set
// re-derives from the same response's latched tier, and a later `local`
// observation clears the refusal without an app restart.
// Owning layer: the app-wide capability state over the shared transport
// (MSW fakes the network; everything between the request and the store is the
// production wiring).
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { apiClient } from "@/lib/api-client";
import { capabilitiesForTier } from "../../lib/gateway-capabilities";
import {
  gatewayCapabilityStore,
  startGatewayCapabilityObserver,
} from "../gateway-capability-store";

const SCHEDULER_PATH = "/api/scheduler";

function loopbackEnvelope(code: string): { error: string; code: string } {
  // Mirrors the daemon's 403 envelope: top-level machine `code` next to the
  // message (`contract.ErrorPayload`, IT-002's wire shape).
  return { error: "this action can only run on the machine running CompozyOS", code };
}

const server = setupServer();

describe("gateway capability store over the real client wiring", () => {
  let stop: () => void;

  beforeEach(() => {
    gatewayCapabilityStore.trigger.capabilitiesReset();
    stop = startGatewayCapabilityObserver();
  });

  afterEach(() => {
    server.resetHandlers();
    stop();
    gatewayCapabilityStore.trigger.capabilitiesReset();
    server.close();
  });

  it("Should record the backstop and re-evaluate capabilities from a loopback 403 envelope", async () => {
    server.use(
      http.get(`*${SCHEDULER_PATH}`, () =>
        HttpResponse.json(loopbackEnvelope("loopback_mutation_required"), {
          status: 403,
          headers: { "X-Compozy-Gateway-Tier": "private" },
        })
      )
    );
    server.listen({ onUnhandledRequest: "bypass" });

    await apiClient.GET(SCHEDULER_PATH);

    const { context } = gatewayCapabilityStore.getSnapshot();
    expect(context.tier).toBe("private");
    // Capabilities re-derived from the refusing response's tier header
    // (stale-map convergence, US-002.EC-2) — the remote read set.
    expect(context.capabilities).toEqual(capabilitiesForTier("private"));
    expect(context.loopbackOnly).toBe("mutation");
  });

  it("Should classify loopback_api_required the same way", async () => {
    server.use(
      http.get(`*${SCHEDULER_PATH}`, () =>
        HttpResponse.json(loopbackEnvelope("loopback_api_required"), {
          status: 403,
          headers: { "X-Compozy-Gateway-Tier": "private" },
        })
      )
    );
    server.listen({ onUnhandledRequest: "bypass" });

    await apiClient.GET(SCHEDULER_PATH);

    expect(gatewayCapabilityStore.getSnapshot().context.loopbackOnly).toBe("api");
  });

  it("Should clear the refusal when a later response latches the local tier", async () => {
    server.use(
      http.get(
        `*${SCHEDULER_PATH}`,
        () =>
          new HttpResponse(JSON.stringify(loopbackEnvelope("loopback_api_required")), {
            status: 403,
            headers: { "X-Compozy-Gateway-Tier": "private" },
          })
      )
    );
    server.listen({ onUnhandledRequest: "bypass" });

    await apiClient.GET(SCHEDULER_PATH);
    expect(gatewayCapabilityStore.getSnapshot().context.loopbackOnly).toBe("api");

    // The operator reconnects at the daemon host: the next observation latches
    // `local` and the stale refusal clears (BR-4, US-002.EC-1 counterpart).
    server.use(
      http.get(`*${SCHEDULER_PATH}`, () =>
        HttpResponse.json({ paused: false }, { headers: { "X-Compozy-Gateway-Tier": "local" } })
      )
    );
    await apiClient.GET(SCHEDULER_PATH);

    const { context } = gatewayCapabilityStore.getSnapshot();
    expect(context.tier).toBe("local");
    expect(context.capabilities).toEqual(capabilitiesForTier("local"));
    expect(context.loopbackOnly).toBeUndefined();
  });
});
