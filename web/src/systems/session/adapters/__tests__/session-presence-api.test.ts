// Suite: session presence HTTP renewal contract
// Invariant: only definitive lease rejection permits replacement; transport and server failures
// remain retryable so the caller does not churn valid presence leases.
// Owning layer: session presence HTTP adapter. No prior adapter suite owned this classification.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse } from "@/test/fetch-test-utils";

import { renewSessionPresence } from "../session-presence-api";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("renewSessionPresence", () => {
  it("Should report a successful renewal", async () => {
    mockEmptyResponse({ status: 204 });

    await expect(renewSessionPresence("ws-alpha", "sess-alpha", "lease-alpha")).resolves.toBe(
      "renewed"
    );
    await expectFetchRequest({
      body: { lease_id: "lease-alpha", visible: true },
      method: "POST",
      path: "/api/workspaces/ws-alpha/sessions/sess-alpha/presence",
    });
  });

  it.each([404, 409])("Should classify status %i as a definitive invalid lease", async status => {
    mockEmptyResponse({ status });

    await expect(renewSessionPresence("ws-alpha", "sess-alpha", "lease-alpha")).resolves.toBe(
      "lease-invalid"
    );
  });

  it.each([500, 503])("Should classify status %i as retryable", async status => {
    mockEmptyResponse({ status });

    await expect(renewSessionPresence("ws-alpha", "sess-alpha", "lease-alpha")).resolves.toBe(
      "retryable-error"
    );
  });

  it("Should classify a transport failure as retryable", async () => {
    vi.mocked(globalThis.fetch).mockRejectedValue(new TypeError("network unavailable"));

    await expect(renewSessionPresence("ws-alpha", "sess-alpha", "lease-alpha")).resolves.toBe(
      "retryable-error"
    );
  });
});
