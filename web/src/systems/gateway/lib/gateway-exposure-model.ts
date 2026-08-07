import type {
  GatewayAddressStatus,
  GatewayReachability,
  GatewayStatus,
  GatewaySurface,
  GatewayTier,
} from "../types";

/**
 * One row of the exposure ladder: a (surface, tier) pair the daemon can turn on
 * or off independently. These are the only exposure decisions the runtime
 * accepts, so the UI offers exactly these and nothing more.
 */
export interface GatewayExposureRow {
  id: string;
  surface: GatewaySurface;
  tier: GatewayTier;
  desired: boolean;
  /** Whether the runtime actually serves this surface right now. */
  served: boolean;
  /** Optimistic-concurrency fence carried back to `POST /api/gateway/surfaces`. */
  generation: number;
  /** Verified reachability of the row's tier — never derived from intent. */
  reachability: GatewayReachability;
  /** Verified addresses for this row's tier. Empty until verification passes. */
  addresses: readonly GatewayAddressStatus[];
  /** Provider failure cause for this tier, redacted by the daemon. */
  cause?: string;
  /** Public operator access is the one row that requires consent on each enable. */
  requiresConsent: boolean;
}

export interface GatewayExposureModel {
  /** `gateway.enabled` — the immutable configuration ceiling. */
  ceilingEnabled: boolean;
  /** True when nothing is reachable beyond this machine. */
  localOnly: boolean;
  rows: readonly GatewayExposureRow[];
  refusal: GatewayStatus["refusal"];
}

const LADDER: ReadonlyArray<{
  id: string;
  surface: GatewaySurface;
  tier: GatewayTier;
  requiresConsent: boolean;
}> = [
  { id: "private-operator-ui", surface: "operator_ui", tier: "private", requiresConsent: false },
  {
    id: "public-webhook-ingress",
    surface: "webhook_ingress",
    tier: "public",
    requiresConsent: false,
  },
  { id: "public-operator-ui", surface: "operator_ui", tier: "public", requiresConsent: true },
];

export function buildGatewayExposureModel(status: GatewayStatus): GatewayExposureModel {
  const rows = LADDER.map(entry => {
    const surface = status.surfaces.find(
      candidate => candidate.surface === entry.surface && candidate.tier === entry.tier
    );
    const served = surface?.observed === "on";
    // Reachability and addresses belong to this row's surface, not to its tier.
    // Two surfaces share the public tier, and one of them being served says
    // nothing about the other: an address listed under a surface the runtime is
    // not serving would tell the operator that traffic arrives there.
    const addresses = served ? liveAddresses(status, entry.tier) : [];
    return {
      ...entry,
      desired: surface?.desired === "enabled",
      served,
      generation: surface?.generation ?? 0,
      reachability: surfaceReachability(status, entry.tier, {
        desired: surface?.desired === "enabled",
        served,
      }),
      addresses,
      ...causeFor(status, entry.tier),
    } satisfies GatewayExposureRow;
  });
  return {
    ceilingEnabled: status.enabled,
    localOnly: isLocalOnly(status),
    rows,
    refusal: status.refusal ?? null,
  };
}

/**
 * Local-only is a runtime fact, not an intent: as long as no tier is advertised
 * and no verified address exists, nothing is reachable from outside — even if
 * the operator has asked for exposure and reconciliation is still running.
 */
export function isLocalOnly(status: GatewayStatus): boolean {
  return !status.tiers.some(tier => tier.advertised) && status.addresses.length === 0;
}

/**
 * Reachability the operator is shown for one tier.
 *
 * `reachable` requires an advertised tier *and* at least one address the daemon
 * marked live, which it only does after challenge verification proves the
 * endpoint reaches this daemon's listener. A provider that merely claims an
 * endpoint therefore reads as `pending`, and a supervised failure reads as
 * `degraded` — never as reachable.
 */
export function tierReachability(status: GatewayStatus, tier: GatewayTier): GatewayReachability {
  const runtime = status.tiers.find(candidate => candidate.tier === tier);
  if (!runtime || runtime.desired !== "enabled") return "off";
  const live = status.addresses.some(address => address.tier === tier && address.live);
  if (runtime.advertised && live) return "reachable";
  if (runtime.observed === "degraded") return "degraded";
  if (status.refusal) return "refused";
  return "pending";
}

/**
 * Reachability the operator is shown for one row of the ladder.
 *
 * `reachable` needs both halves of the truth: the runtime must actually serve
 * *this* surface, and its tier must be advertised with an address the daemon
 * verified. A public tier carrying live webhook ingress therefore leaves the
 * public operator row at `pending` while that surface is off, instead of
 * borrowing its neighbour's reachability.
 */
export function surfaceReachability(
  status: GatewayStatus,
  tier: GatewayTier,
  surface: { desired: boolean; served: boolean }
): GatewayReachability {
  if (!surface.desired) return "off";
  const reachability = tierReachability(status, tier);
  if (reachability === "reachable" && !surface.served) return "pending";
  // A surface the operator asked for is never "off": if its tier carries no
  // provider yet, it is still waiting on one.
  return reachability === "off" ? "pending" : reachability;
}

/** Verified, live addresses for a tier. An unverified endpoint is never listed. */
export function liveAddresses(
  status: GatewayStatus,
  tier: GatewayTier
): readonly GatewayAddressStatus[] {
  return status.addresses.filter(address => address.tier === tier && address.live);
}

function causeFor(status: GatewayStatus, tier: GatewayTier): { cause?: string } {
  const provider = status.providers.find(
    candidate => candidate.tier === tier && candidate.desired === "enabled"
  );
  const cause = provider?.cause?.trim();
  return cause ? { cause } : {};
}
