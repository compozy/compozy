import type { ExtensionEntry } from "@/systems/extensions";

import type { GatewayAuditReport, GatewayDevice, GatewayStatus, GatewayTier } from "../types";

/**
 * An installed extension that declares the connectivity-provider capability.
 * Defaults are the ready case: enabled, network participation confirmed, and
 * every declared secret bound.
 */
export function connectivityProviderFixture(
  overrides: Partial<ExtensionEntry> = {}
): ExtensionEntry {
  return {
    capabilities: ["connectivity.provider"],
    consecutive_failures: 0,
    daemon_running: true,
    digest_matched: true,
    enabled: true,
    health: "healthy",
    missing_env: [],
    name: "connectivity-overlay",
    network_confirmation_required: false,
    network_requirement_digest: "sha256:overlay-control-digest",
    requires_env: ["OVERLAY_AUTH_KEY"],
    restart_backoff_ms: 0,
    source: "bundled",
    state: "running",
    type: "backend",
    update_available: false,
    version: "0.1.0",
    ...overrides,
  };
}

export function gatewayDeviceFixture(overrides: Partial<GatewayDevice> = {}): GatewayDevice {
  return {
    actor_kind: "operator_device",
    created_at: "2026-08-01T09:00:00Z",
    id: "dev_phone",
    last_seen_at: "2026-08-07T08:00:00Z",
    name: "Work phone",
    pairing_origin: "private",
    revoke_epoch: 0,
    revoked_at: null,
    ...overrides,
  };
}

/** Local-only posture: nothing desired, nothing advertised, no addresses. */
export function gatewayStatusFixture(overrides: Partial<GatewayStatus> = {}): GatewayStatus {
  return {
    addresses: [],
    bindings: [],
    changed: false,
    devices: [],
    enabled: true,
    providers: [],
    refusal: null,
    surfaces: [],
    tiers: [
      { advertised: false, desired: "disabled", observed: "down", tier: "private" },
      { advertised: false, desired: "disabled", observed: "down", tier: "public" },
    ],
    ...overrides,
  };
}

/**
 * A tier the operator asked for, at a chosen point in its lifecycle. `verified`
 * is what separates "the provider claims an endpoint" from "the daemon proved
 * the endpoint reaches this listener".
 */
export function gatewayTierFixture(input: {
  tier: GatewayTier;
  observed: "down" | "establishing" | "up" | "degraded";
  verified: boolean;
  surface?: "operator_ui" | "webhook_ingress";
  served?: boolean;
  generation?: number;
  cause?: string;
}): Partial<GatewayStatus> {
  const surface = input.surface ?? "operator_ui";
  const address = `https://${input.tier}.gateway.test`;
  return {
    addresses: input.verified ? [{ address, live: true, tier: input.tier }] : [],
    providers: [
      {
        cause: input.cause ?? "",
        desired: "enabled",
        generation: 1,
        health: input.observed === "up" ? "healthy" : input.observed,
        name: "connectivity-fixture",
        observed: input.observed,
        tier: input.tier,
      },
    ],
    surfaces: [
      {
        desired: "enabled",
        generation: input.generation ?? 3,
        observed: (input.served ?? input.verified) ? "on" : "off",
        surface,
        tier: input.tier,
      },
    ],
    tiers: [
      {
        advertised: input.verified,
        desired: "enabled",
        observed: input.observed,
        tier: input.tier,
        ...(input.verified ? { listener_address: "127.0.0.1:41111" } : {}),
      },
      {
        advertised: false,
        desired: "disabled",
        observed: "down",
        tier: input.tier === "private" ? "public" : "private",
      },
    ],
  };
}

export function gatewayAuditFixture(
  overrides: Partial<GatewayAuditReport> = {}
): GatewayAuditReport {
  return {
    auth: { active_device_count: 1, mode: "device_session", required_remotely: true },
    device_highlights: {
      active: 1,
      recently_paired: 0,
      revoked: 0,
      stale: 0,
      stale_device_ids: [],
    },
    findings: [],
    local_only: true,
    no_findings: true,
    ran: true,
    status: gatewayStatusFixture(),
    ...overrides,
  };
}
