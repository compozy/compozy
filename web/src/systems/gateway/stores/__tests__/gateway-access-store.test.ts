// Suite: gateway access store
// Invariant: this browser's device standing changes only on an explicit daemon decision. Transient
// failures — dropped connections, timeouts, 5xx, unrelated 401s — never eject the operator, and a
// revocation is terminal against a later generic unauthenticated response.
// Owning layer: the app-wide access state between the shared transport and the access boundary.
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  classifyGatewayAccess,
  reportGatewayAccess,
  reportGatewayListenerTier,
} from "@/lib/gateway-access-signal";

import { gatewayAccessStore, startGatewayAccessObserver } from "../gateway-access-store";

describe("gateway access classification", () => {
  it("Should recognise only the two explicit daemon decisions", () => {
    expect(classifyGatewayAccess(401, { code: "gateway_device_unauthenticated" })).toBe(
      "unauthenticated"
    );
    expect(classifyGatewayAccess(401, { code: "gateway_device_revoked" })).toBe("revoked");
  });

  it("Should ignore a rejected stream ticket, which is resolved by minting a fresh one", () => {
    expect(classifyGatewayAccess(401, { code: "gateway_stream_ticket_invalid" })).toBeUndefined();
  });

  it("Should ignore transient failures and unrelated responses", () => {
    expect(classifyGatewayAccess(503, { code: "gateway_device_revoked" })).toBeUndefined();
    expect(classifyGatewayAccess(500, { error: "boom" })).toBeUndefined();
    expect(classifyGatewayAccess(0, undefined)).toBeUndefined();
    expect(classifyGatewayAccess(401, { error: "Unauthorized" })).toBeUndefined();
    expect(classifyGatewayAccess(401, null)).toBeUndefined();
  });
});

describe("gateway access store", () => {
  let stop: () => void;

  beforeEach(() => {
    gatewayAccessStore.trigger.accessRestored();
    stop = startGatewayAccessObserver();
  });

  afterEach(() => {
    stop();
    gatewayAccessStore.trigger.accessRestored();
  });

  it("Should start with access intact", () => {
    expect(gatewayAccessStore.getSnapshot().context.state).toBe("ok");
  });

  it("Should not move on a network or server failure", () => {
    reportGatewayAccess(503, { error: "temporarily unavailable" });
    reportGatewayAccess(500, { code: "gateway_device_revoked" });
    expect(gatewayAccessStore.getSnapshot().context.state).toBe("ok");
  });

  it("Should show the pairing gate when the daemon says this device is unauthenticated", () => {
    reportGatewayListenerTier("public");
    reportGatewayAccess(401, { code: "gateway_device_unauthenticated" });
    expect(gatewayAccessStore.getSnapshot().context).toMatchObject({
      state: "unauthenticated",
      tier: "public",
    });
  });

  it("Should keep a revocation terminal against a later generic unauthenticated response", () => {
    reportGatewayAccess(401, { code: "gateway_device_revoked" });
    reportGatewayAccess(401, { code: "gateway_device_unauthenticated" });
    expect(gatewayAccessStore.getSnapshot().context.state).toBe("revoked");
  });

  it("Should restore access after a successful redeem", () => {
    reportGatewayAccess(401, { code: "gateway_device_revoked" });
    gatewayAccessStore.trigger.accessRestored();
    expect(gatewayAccessStore.getSnapshot().context.state).toBe("ok");
  });

  it("Should keep exactly one subscription when started more than once", () => {
    const second = startGatewayAccessObserver();
    second();
    reportGatewayAccess(401, { code: "gateway_device_unauthenticated" });
    expect(gatewayAccessStore.getSnapshot().context.state).toBe("unauthenticated");
  });
});
