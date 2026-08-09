import type { ExtensionEntry } from "@/systems/extensions";

import type { GatewayProviderStatus, GatewayStatus, GatewayTier } from "../types";

/** The capability an extension must declare to serve daemon reachability. */
export const CONNECTIVITY_PROVIDER_CAPABILITY = "connectivity.provider";

/** Extension source the gateway refuses outright, per the live trust resolver. */
const FORBIDDEN_SOURCE = "workspace";

export type GatewayProviderHealth = "healthy" | "degraded" | "down";
export type GatewayProviderObservedState = "down" | "establishing" | "up" | "degraded";

export interface GatewayProviderActivation extends Omit<
  GatewayProviderStatus,
  "health" | "observed" | "tier"
> {
  health: GatewayProviderHealth | null;
  observed: GatewayProviderObservedState | null;
  tier: GatewayTier | null;
}

/**
 * Something that must be true before the daemon will authorize this provider,
 * with the action that satisfies it. Every blocker is read from the extension's
 * own payload — nothing here is assumed about any particular provider.
 */
export interface GatewayProviderBlocker {
  id: string;
  message: string;
}

export interface GatewayProviderCandidate {
  name: string;
  /** `install_source` the enable call must carry; re-derived by the daemon. */
  installSource: string;
  /** Control digest the enable call confirms; empty when the extension declares none. */
  digest?: string;
  blockers: readonly GatewayProviderBlocker[];
  /** Tiers this provider is currently activated on, from gateway status. */
  activations: readonly GatewayProviderActivation[];
}

export interface GatewayProviderModel {
  candidates: readonly GatewayProviderCandidate[];
  /**
   * Activations whose extension is no longer installed. The daemon still holds
   * durable intent for them, so they stay visible and disableable.
   */
  orphanedActivations: readonly GatewayProviderActivation[];
  /** No installed connectivity provider and no durable activation. */
  isEmpty: boolean;
}

/** Installed extensions that declare the connectivity-provider capability. */
export function connectivityProviderExtensions(
  extensions: readonly ExtensionEntry[]
): readonly ExtensionEntry[] {
  return extensions.filter(extension =>
    extension.capabilities?.includes(CONNECTIVITY_PROVIDER_CAPABILITY)
  );
}

/**
 * Joins durable gateway activations with the live extension inventory.
 *
 * The daemon re-derives install source and control digest from the live
 * registry on every enable, so this model only reports what the operator must
 * do first — it never predicts authorization.
 */
export function buildGatewayProviderModel(
  status: GatewayStatus,
  extensions: readonly ExtensionEntry[]
): GatewayProviderModel {
  const installed = connectivityProviderExtensions(extensions);
  const activationsByProvider = new Map<string, GatewayProviderActivation[]>();
  const normalizedActivations: GatewayProviderActivation[] = [];
  for (const provider of status.providers) {
    const activation = normalizeProviderActivation(provider);
    normalizedActivations.push(activation);
    const activations = activationsByProvider.get(provider.name);
    if (activations) {
      activations.push(activation);
    } else {
      activationsByProvider.set(provider.name, [activation]);
    }
  }

  const candidates = installed.map(extension => {
    const digest = extension.network_requirement_digest?.trim();
    return {
      name: extension.name,
      installSource: extension.source,
      ...(digest ? { digest } : {}),
      blockers: providerBlockers(extension),
      activations: activationsByProvider.get(extension.name) ?? [],
    } satisfies GatewayProviderCandidate;
  });
  const known = new Set(installed.map(extension => extension.name));
  const orphanedActivations: GatewayProviderActivation[] = [];
  for (const activation of normalizedActivations) {
    if (!known.has(activation.name)) orphanedActivations.push(activation);
  }
  return {
    candidates,
    orphanedActivations,
    isEmpty: candidates.length === 0 && orphanedActivations.length === 0,
  };
}

/** The activation for one tier, or `undefined` when the provider does not serve it. */
export function activationForTier(
  candidate: GatewayProviderCandidate,
  tier: GatewayTier
): GatewayProviderActivation | undefined {
  return candidate.activations.find(activation => activation.tier === tier);
}

/** The optimistic-concurrency fence for this provider on this tier. */
export function providerGeneration(candidate: GatewayProviderCandidate, tier: GatewayTier): number {
  return activationForTier(candidate, tier)?.generation ?? 0;
}

function providerBlockers(extension: ExtensionEntry): readonly GatewayProviderBlocker[] {
  const blockers: GatewayProviderBlocker[] = [];
  if (extension.source === FORBIDDEN_SOURCE) {
    blockers.push({
      id: "workspace-source",
      message:
        "This provider belongs to one workspace. Remote reachability applies to the whole daemon, so install it globally in Settings → Extensions.",
    });
    return blockers;
  }
  if (!extension.enabled) {
    blockers.push({
      id: "extension-disabled",
      message: `Enable the ${extension.name} extension in Settings → Extensions first.`,
    });
  }
  if (extension.network_confirmation_required) {
    blockers.push({
      id: "network-confirmation",
      message: `Confirm what ${extension.name} may reach, in Settings → Extensions.`,
    });
  }
  const missing = extension.missing_env ?? [];
  if (missing.length > 0) {
    blockers.push({
      id: "missing-secrets",
      message: `Bind ${missing.join(", ")} for ${extension.name} in Settings → Extensions.`,
    });
  }
  return blockers;
}

function normalizeProviderActivation(provider: GatewayProviderStatus): GatewayProviderActivation {
  return {
    ...provider,
    health: parseProviderHealth(provider.health),
    observed: parseProviderObservedState(provider.observed),
    tier: parseGatewayTier(provider.tier),
  };
}

function parseGatewayTier(value: string): GatewayTier | null {
  return value === "private" || value === "public" ? value : null;
}

function parseProviderHealth(value: string): GatewayProviderHealth | null {
  return value === "healthy" || value === "degraded" || value === "down" ? value : null;
}

function parseProviderObservedState(value: string): GatewayProviderObservedState | null {
  return value === "down" || value === "establishing" || value === "up" || value === "degraded"
    ? value
    : null;
}
