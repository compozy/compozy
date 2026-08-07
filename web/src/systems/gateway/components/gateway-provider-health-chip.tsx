import type { GatewaySignalTone } from "../lib/gateway-copy";
import type { GatewayProviderStatus } from "../types";
import { GatewayStatusChip } from "./gateway-status-chip";

/**
 * Health for one provider activation, straight from gateway status.
 *
 * `health` and `observed` answer different questions — whether the provider is
 * responding, and how far the reconciler got — so both are shown rather than
 * collapsed into one word.
 */
export function GatewayProviderHealthChip({ activation }: { activation: GatewayProviderStatus }) {
  const copy = PROVIDER_HEALTH_COPY[activation.health] ?? {
    label: activation.health,
    tone: "neutral" as GatewaySignalTone,
  };
  return (
    <span className="inline-flex min-w-0 flex-col items-end gap-0.5">
      <GatewayStatusChip
        data-testid={`gateway-provider-health-${activation.name}-${activation.tier}`}
        detail={activation.observed}
        label={copy.label}
        tone={copy.tone}
      />
      {activation.cause ? (
        <span
          className="max-w-64 text-right text-form-label text-danger"
          data-testid={`gateway-provider-cause-${activation.name}-${activation.tier}`}
        >
          {activation.cause}
        </span>
      ) : null}
    </span>
  );
}

const PROVIDER_HEALTH_COPY: Record<string, { label: string; tone: GatewaySignalTone }> = {
  healthy: { label: "Healthy", tone: "success" },
  degraded: { label: "Degraded", tone: "warning" },
  down: { label: "Down", tone: "danger" },
};
