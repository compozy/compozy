import type { GatewaySignalTone } from "../lib/gateway-copy";
import type { GatewayProviderActivation } from "../lib/gateway-provider-model";
import { GatewayStatusChip } from "./gateway-status-chip";

export interface GatewayProviderHealthChipProps {
  activation: GatewayProviderActivation;
}

/**
 * Health for one provider activation, straight from gateway status.
 *
 * `health` and `observed` answer different questions — whether the provider is
 * responding, and how far the reconciler got — so both are shown rather than
 * collapsed into one word.
 */
export function GatewayProviderHealthChip({ activation }: GatewayProviderHealthChipProps) {
  const copy = activation.health
    ? PROVIDER_HEALTH_COPY[activation.health]
    : { label: "Unknown", tone: "neutral" as GatewaySignalTone };
  const observed = activation.observed
    ? PROVIDER_OBSERVED_COPY[activation.observed]
    : "State unavailable";
  const tier = activation.tier ?? "unknown";
  return (
    <span className="inline-flex min-w-0 flex-col items-end gap-0.5">
      <GatewayStatusChip
        data-testid={`gateway-provider-health-${activation.name}-${tier}`}
        detail={observed}
        label={copy.label}
        tone={copy.tone}
      />
      {activation.cause ? (
        <span
          className="max-w-64 text-right text-form-label text-danger"
          data-testid={`gateway-provider-cause-${activation.name}-${tier}`}
        >
          {activation.cause}
        </span>
      ) : null}
    </span>
  );
}

const PROVIDER_HEALTH_COPY = {
  healthy: { label: "Healthy", tone: "success" },
  degraded: { label: "Degraded", tone: "warning" },
  down: { label: "Down", tone: "danger" },
} satisfies Record<string, { label: string; tone: GatewaySignalTone }>;

const PROVIDER_OBSERVED_COPY = {
  down: "Down",
  establishing: "Establishing",
  up: "Up",
  degraded: "Degraded",
} as const;
