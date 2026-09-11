// Suite: gateway access signal ordering
// Invariant: an explicit gateway 401 has already ended the session by the moment the failing call
// returns, so the caller cannot render or reuse protected cached data first. Every other 401 and
// every transient failure leaves the session untouched.
// Owning layer: the shared api-client response middleware.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiClient } from "@/lib/api-client";
import {
  observeGatewayAccess,
  observeGatewayListenerTier,
  type GatewayAccessSignal,
  type GatewayListenerTier,
} from "@/lib/gateway-access-signal";

function jsonResponse(status: number, payload: unknown, tier?: GatewayListenerTier): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json",
      ...(tier ? { "X-Compozy-Gateway-Tier": tier } : {}),
    },
  });
}

/**
 * The transport classifies by response URL, but `Response.url` is set by fetch
 * and cannot be passed to the constructor — shadow the read-only getter with
 * the URL a real daemon response would carry.
 */
function withUrl(response: Response, url: string): Response {
  Object.defineProperty(response, "url", { value: url });
  return response;
}

describe("gateway access signal ordering", () => {
  let signals: GatewayAccessSignal[];
  let tiers: (GatewayListenerTier | undefined)[];
  let unobserve: () => void;
  let unobserveTier: () => void;

  beforeEach(() => {
    signals = [];
    tiers = [];
    unobserve = observeGatewayAccess(signal => signals.push(signal));
    unobserveTier = observeGatewayListenerTier(tier => tiers.push(tier));
  });

  afterEach(() => {
    unobserve();
    unobserveTier();
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
          jsonResponse(
            401,
            { error: "Unauthorized", code: "gateway_device_unauthenticated" },
            "public"
          )
        )
    );

    await apiClient.GET("/api/gateway/status");

    expect(signals).toEqual(["unauthenticated"]);
    expect(tiers).toEqual(["public"]);
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

// Suite: gateway status tier latch
// Invariant: `/api/status` is the tier's authoritative source — a response from
// it without a parsable tier header publishes unknown (US-001.EC-2, so the
// default-hidden capability set applies, BR-2) instead of keeping a stale
// latched tier. Any other response publishes only a parsable tier: random
// non-status responses must never unlatch the shell.
// Owning layer: the shared api-client response middleware.
describe("gateway status tier latch", () => {
  let tiers: (GatewayListenerTier | undefined)[];
  let unobserveTier: () => void;

  beforeEach(() => {
    tiers = [];
    unobserveTier = observeGatewayListenerTier(tier => tiers.push(tier));
  });

  afterEach(() => {
    unobserveTier();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  /** A `/api/status` response with an optional raw (possibly unparsable) tier header. */
  function statusResponse(status: number, rawTier?: string): Response {
    return withUrl(
      new Response(JSON.stringify({}), {
        status,
        headers: {
          "Content-Type": "application/json",
          ...(rawTier !== undefined ? { "X-Compozy-Gateway-Tier": rawTier } : {}),
        },
      }),
      "https://compozy.local/api/status"
    );
  }

  it("Should latch unknown when /api/status answers without a tier header", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(statusResponse(200)));

    await apiClient.GET("/api/status");

    expect(tiers).toEqual([undefined]);
  });

  it("Should latch unknown when the /api/status tier header does not parse", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(statusResponse(200, "interior")));

    await apiClient.GET("/api/status");

    expect(tiers).toEqual([undefined]);
  });

  it("Should unlatch a stale tier when /api/status stops carrying a parsable one", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(200, {}, "local")));
    await apiClient.GET("/api/gateway/status");
    expect(tiers).toEqual(["local"]);

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(statusResponse(200)));
    await apiClient.GET("/api/status");

    expect(tiers).toEqual(["local", undefined]);
  });

  it("Should keep the last latched tier when other responses omit the header", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(200, {}, "local")));
    await apiClient.GET("/api/gateway/status");
    expect(tiers).toEqual(["local"]);

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(200, {})));
    await apiClient.GET("/api/gateway/status");

    expect(tiers).toEqual(["local"]);
  });

  it("Should latch a parsable tier from /api/status", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(statusResponse(200, "private")));

    await apiClient.GET("/api/status");

    expect(tiers).toEqual(["private"]);
  });
});
