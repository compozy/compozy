import { apiClient, apiRequestFailed } from "@/lib/api-client";

import type {
  GatewayAuditReport,
  GatewayDevice,
  GatewayPairingArtifact,
  GatewayProviderEnableRequest,
  GatewayRedeemRequest,
  GatewayRedeemResult,
  GatewayRevokeResult,
  GatewayStatus,
  GatewaySurfaceRequest,
  GatewayTier,
} from "../types";
import { GatewayApiError, gatewayApiError } from "./gateway-api-error";

function requireGatewayData<T>(data: T | undefined, response: Response, fallback: string): T {
  if (data === undefined) {
    throw new GatewayApiError(`${fallback}: empty response (${response.status})`, response.status);
  }
  return data;
}

async function fetchGatewayStatus(signal?: AbortSignal): Promise<GatewayStatus> {
  const { data, error, response } = await apiClient.GET("/api/gateway/status", { signal });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError("Failed to read gateway status", response, error);
  }
  return requireGatewayData(data, response, "Failed to read gateway status");
}

async function fetchGatewayAudit(signal?: AbortSignal): Promise<GatewayAuditReport> {
  const { data, error, response } = await apiClient.GET("/api/gateway/audit", { signal });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError("Failed to run the gateway audit", response, error);
  }
  return requireGatewayData(data, response, "Failed to run the gateway audit");
}

async function listGatewayDevices(signal?: AbortSignal): Promise<GatewayDevice[]> {
  const { data, error, response } = await apiClient.GET("/api/gateway/devices", { signal });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError("Failed to list paired devices", response, error);
  }
  return requireGatewayData(data, response, "Failed to list paired devices").devices;
}

async function renameGatewayDevice(
  id: string,
  name: string,
  signal?: AbortSignal
): Promise<GatewayDevice> {
  const { data, error, response } = await apiClient.PATCH("/api/gateway/devices/{id}", {
    params: { path: { id } },
    body: { name },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError(`Failed to rename device "${id}"`, response, error);
  }
  return requireGatewayData(data, response, `Failed to rename device "${id}"`);
}

async function revokeGatewayDevice(id: string, signal?: AbortSignal): Promise<GatewayRevokeResult> {
  const { data, error, response } = await apiClient.DELETE("/api/gateway/devices/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError(`Failed to revoke device "${id}"`, response, error);
  }
  return requireGatewayData(data, response, `Failed to revoke device "${id}"`);
}

/**
 * Mints a bounded, single-use pairing artifact. The raw artifact exists only in
 * this response — it is never re-readable, so the issuance view is the one and
 * only place it may appear.
 */
async function mintGatewayPairing(signal?: AbortSignal): Promise<GatewayPairingArtifact> {
  const { data, error, response } = await apiClient.POST("/api/gateway/pairings", { signal });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError("Failed to mint a pairing code", response, error);
  }
  return requireGatewayData(data, response, "Failed to mint a pairing code");
}

async function redeemGatewayPairing(
  body: GatewayRedeemRequest,
  signal?: AbortSignal
): Promise<GatewayRedeemResult> {
  const { data, error, response } = await apiClient.POST("/api/gateway/pairings/redeem", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError("Failed to redeem the pairing code", response, error);
  }
  return requireGatewayData(data, response, "Failed to redeem the pairing code");
}

/**
 * Sets durable exposure for one surface on one tier. `consent` is a per-call
 * boolean: the daemon requires it again on every enable of the public operator
 * UI, so it is never cached or replayed.
 */
async function setGatewaySurface(
  body: GatewaySurfaceRequest,
  signal?: AbortSignal
): Promise<GatewayStatus> {
  const { data, error, response } = await apiClient.POST("/api/gateway/surfaces", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError("Failed to change gateway exposure", response, error);
  }
  return requireGatewayData(data, response, "Failed to change gateway exposure");
}

async function enableGatewayProvider(
  name: string,
  body: GatewayProviderEnableRequest,
  signal?: AbortSignal
): Promise<GatewayStatus> {
  const { data, error, response } = await apiClient.POST("/api/gateway/providers/{name}/enable", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError(`Failed to enable provider "${name}"`, response, error);
  }
  return requireGatewayData(data, response, `Failed to enable provider "${name}"`);
}

async function disableGatewayProvider(
  name: string,
  tier: GatewayTier,
  signal?: AbortSignal
): Promise<GatewayStatus> {
  const { data, error, response } = await apiClient.POST("/api/gateway/providers/{name}/disable", {
    params: { path: { name }, query: { tier } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw gatewayApiError(`Failed to disable provider "${name}"`, response, error);
  }
  return requireGatewayData(data, response, `Failed to disable provider "${name}"`);
}

export const gatewayApi = {
  status: fetchGatewayStatus,
  audit: fetchGatewayAudit,
  listDevices: listGatewayDevices,
  renameDevice: renameGatewayDevice,
  revokeDevice: revokeGatewayDevice,
  mintPairing: mintGatewayPairing,
  redeemPairing: redeemGatewayPairing,
  setSurface: setGatewaySurface,
  enableProvider: enableGatewayProvider,
  disableProvider: disableGatewayProvider,
};
