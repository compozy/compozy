// Suite: gateway access signal ordering
// Invariant: an explicit gateway 401 has already ended the session by the moment the failing call
// returns, so the caller cannot render or reuse protected cached data first. Every other 401 and
// every transient failure leaves the session untouched.
// Owning layer: the shared api-client response middleware.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiClient } from "@/lib/api-client";
import { observeGatewayAccess, type GatewayAccessSignal } from "@/lib/gateway-access-signal";

function jsonResponse(status: number, payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("gateway access signal ordering", () => {
  let signals: GatewayAccessSignal[];
  let unobserve: () => void;

  beforeEach(() => {
    signals = [];
    unobserve = observeGatewayAccess(signal => signals.push(signal));
  });

  afterEach(() => {
    unobserve();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should have ended the session before a revoked call returns to its caller", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse(401, { error: "Unauthorized", code: "gateway_device_revoked" })
        )
    );

    const result = await apiClient.GET("/api/gateway/status");

    // Asserted synchronously after the await: no extra tick was needed, so a
    // caller reacting to this failure cannot beat the terminal signal.
    expect(signals).toEqual(["revoked"]);
    expect(result.response.status).toBe(401);
  });

  it("Should have ended the session before an unauthenticated call returns", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse(401, { error: "Unauthorized", code: "gateway_device_unauthenticated" })
        )
    );

    await apiClient.GET("/api/gateway/status");

    expect(signals).toEqual(["unauthenticated"]);
  });

  it("Should leave the session untouched for a 401 that carries no gateway decision", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(401, { error: "Unauthorized" })));

    await apiClient.GET("/api/gateway/status");

    expect(signals).toEqual([]);
  });

  it("Should leave the session untouched for a 401 without a JSON envelope", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("nope", { status: 401 })));

    await apiClient.GET("/api/gateway/status");

    expect(signals).toEqual([]);
  });

  it("Should leave the session untouched for a transient server failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(503, { code: "gateway_device_revoked" }))
    );

    await apiClient.GET("/api/gateway/status");

    expect(signals).toEqual([]);
  });
});
