// Suite: gateway HTTP adapter
// Invariant: every gateway operation sends its generated wire contract and reports failures as
// GatewayApiError with the daemon status and machine code intact.
// Owning layer: the gateway adapter at the openapi-fetch boundary.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse, mockJsonResponse } from "@/test/fetch-test-utils";

import {
  gatewayAuditFixture,
  gatewayDeviceFixture,
  gatewayStatusFixture,
} from "../../mocks/fixtures";
import { gatewayApi } from "../gateway-api";
import { GatewayApiError } from "../gateway-api-error";

const device = gatewayDeviceFixture();
const status = gatewayStatusFixture({ devices: [device] });
const audit = gatewayAuditFixture({ status });
const artifact = {
  artifact: "cpz_gwp_2f8c1d5a9b4e7c0d",
  expires_at: "2099-01-01T00:00:00Z",
};
const revoke = { canceled: 1, changed: true, device };
const redeem = { device, credential: "credential" };
const surfaceBody = {
  consent: false,
  desired: "enabled" as const,
  expected_generation: 3,
  surface: "operator_ui" as const,
  tier: "private" as const,
};
const enableBody = {
  digest_confirmed: "sha256:control",
  expected_generation: 2,
  install_source: "bundled",
  tier: "private" as const,
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("gatewayApi", () => {
  it.each([
    {
      name: "status",
      response: status,
      run: () => gatewayApi.status(),
      expected: status,
      request: { path: "/api/gateway/status" },
    },
    {
      name: "audit",
      response: audit,
      run: () => gatewayApi.audit(),
      expected: audit,
      request: { path: "/api/gateway/audit" },
    },
    {
      name: "device inventory",
      response: { devices: [device] },
      run: () => gatewayApi.listDevices(),
      expected: [device],
      request: { path: "/api/gateway/devices" },
    },
    {
      name: "device rename",
      response: device,
      run: () => gatewayApi.renameDevice("dev_phone", "Work phone"),
      expected: device,
      request: {
        body: { name: "Work phone" },
        method: "PATCH",
        path: "/api/gateway/devices/dev_phone",
      },
    },
    {
      name: "device revoke",
      response: revoke,
      run: () => gatewayApi.revokeDevice("dev_phone"),
      expected: revoke,
      request: { method: "DELETE", path: "/api/gateway/devices/dev_phone" },
    },
    {
      name: "pairing mint",
      response: artifact,
      status: 201,
      run: () => gatewayApi.mintPairing(),
      expected: artifact,
      request: { method: "POST", path: "/api/gateway/pairings" },
    },
    {
      name: "pairing redeem",
      response: redeem,
      run: () =>
        gatewayApi.redeemPairing({
          actor_kind: "operator_device",
          artifact: artifact.artifact,
          name: "Work phone",
        }),
      expected: redeem,
      request: {
        body: {
          actor_kind: "operator_device",
          artifact: artifact.artifact,
          name: "Work phone",
        },
        method: "POST",
        path: "/api/gateway/pairings/redeem",
      },
    },
    {
      name: "surface transition",
      response: status,
      run: () => gatewayApi.setSurface(surfaceBody),
      expected: status,
      request: { body: surfaceBody, method: "POST", path: "/api/gateway/surfaces" },
    },
    {
      name: "provider enable",
      response: status,
      run: () => gatewayApi.enableProvider("connectivity-overlay", enableBody),
      expected: status,
      request: {
        body: enableBody,
        method: "POST",
        path: "/api/gateway/providers/connectivity-overlay/enable",
      },
    },
    {
      name: "provider disable",
      response: status,
      run: () => gatewayApi.disableProvider("connectivity-overlay", "private"),
      expected: status,
      request: {
        method: "POST",
        path: "/api/gateway/providers/connectivity-overlay/disable?tier=private",
      },
    },
  ])("Should send the $name wire contract", async testCase => {
    mockJsonResponse(testCase.response, { status: testCase.status ?? 200 });

    await expect(testCase.run()).resolves.toEqual(testCase.expected);
    await expectFetchRequest(testCase.request);
  });

  it("Should preserve a daemon failure code in GatewayApiError", async () => {
    mockJsonResponse(
      { error: "Fresh consent is required", code: "gateway_consent_required" },
      { status: 409 }
    );

    await expect(gatewayApi.setSurface(surfaceBody)).rejects.toMatchObject({
      name: "GatewayApiError",
      message: "Fresh consent is required",
      status: 409,
      code: "gateway_consent_required",
    });
  });

  it("Should map a malformed error envelope to the operation fallback", async () => {
    mockJsonResponse({ unexpected: true }, { status: 500 });

    await expect(gatewayApi.status()).rejects.toMatchObject({
      name: "GatewayApiError",
      message: "Failed to read gateway status: 500",
      status: 500,
    });
  });

  it("Should reject an empty success payload with GatewayApiError", async () => {
    mockEmptyResponse();

    await expect(gatewayApi.status()).rejects.toBeInstanceOf(GatewayApiError);
  });
});
