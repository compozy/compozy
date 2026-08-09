import type {
  GatewayIngressReachability,
  GatewayReachability,
  GatewaySurface,
  GatewayTier,
} from "../types";
import type { GatewayExposureRowId } from "./gateway-exposure-model";

/**
 * Named exposure modes with the risk each one carries, in the operator's own
 * words. The ladder is never presented as a bare toggle: the person deciding
 * has to be able to read what becomes reachable and by whom.
 */
export const EXPOSURE_ROW_COPY: Record<GatewayExposureRowId, { title: string; risk: string }> = {
  "private-operator-ui": {
    title: "Private overlay",
    risk: "Devices you have paired reach the full product over your own overlay network. Nothing is published to the internet.",
  },
  "public-webhook-ingress": {
    title: "Public delivery ingress",
    risk: "A public address accepts signed webhook and bridge deliveries only. Each delivery still has to pass its own signature and freshness checks.",
  },
  "public-operator-ui": {
    title: "Public operator access",
    risk: "The full operator UI and management API answer on the public address. Anyone who reaches it still needs a paired device, and pairing is never offered there.",
  },
};

/**
 * The concrete disclosure shown before the public operator UI is enabled. Each
 * line names something that actually becomes reachable — no summaries.
 */
export const PUBLIC_OPERATOR_CONSENT_DISCLOSURE: readonly string[] = [
  "Your public address starts serving the operator UI and the whole management API — sessions, tasks, loops, memory, settings, bridges and extensions.",
  "Anyone who reaches that address sees the pairing gate. Only a device you paired from the private overlay or this machine gets past it.",
  "Pairing codes are never minted or redeemed on the public address.",
  "Turning this off takes effect immediately, and a restart never turns it back on.",
];

/** Shown wherever a public delivery URL is displayed. */
export const DELIVERY_DURABILITY_NOTE =
  "Deliveries reach Compozy only while the daemon and its connectivity provider are running. There is no queue for messages sent while it is offline — configure retries at the sender.";

export const TIER_LABEL: Record<GatewayTier, string> = {
  private: "Private overlay",
  public: "Public address",
};

export const SURFACE_LABEL: Record<GatewaySurface, string> = {
  operator_ui: "Operator UI and management API",
  webhook_ingress: "Webhook and bridge delivery",
};

/** Status chip tones map to the signal palette: information, never decoration. */
export type GatewaySignalTone = "success" | "warning" | "danger" | "info" | "neutral";

export const REACHABILITY_COPY: Record<
  GatewayReachability,
  { label: string; tone: GatewaySignalTone; detail: string }
> = {
  off: {
    label: "Off",
    tone: "neutral",
    detail: "Not reachable from anywhere but this machine.",
  },
  pending: {
    label: "Establishing",
    tone: "info",
    detail: "Waiting for the connectivity provider to publish an address Compozy can verify.",
  },
  degraded: {
    label: "Degraded",
    tone: "warning",
    detail: "The connectivity provider failed. Nothing is being advertised while it recovers.",
  },
  reachable: {
    label: "Reachable",
    tone: "success",
    detail: "Verified: this address was proven to reach this daemon.",
  },
  refused: {
    label: "Refused",
    tone: "danger",
    detail: "The safety policy refused this exposure.",
  },
};

export const INGRESS_REACHABILITY_COPY: Record<
  GatewayIngressReachability,
  { label: string; tone: GatewaySignalTone; detail: string }
> = {
  off: {
    label: "Off",
    tone: "neutral",
    detail: "Public delivery ingress is turned off, so this URL would not answer.",
  },
  unconfirmed: {
    label: "Unconfirmed",
    tone: "warning",
    detail: "Confirm this subject against the current public address before publishing the URL.",
  },
  live: {
    label: "Live",
    tone: "success",
    detail: "Confirmed against the verified public address.",
  },
  broken: {
    label: "Broken",
    tone: "danger",
    detail: "The public address is no longer reachable.",
  },
  reconfirmation_required: {
    label: "Reconfirm",
    tone: "warning",
    detail: "The public address changed. Confirm again before senders use this URL.",
  },
};

export const AUDIT_SEVERITY_COPY: Record<string, { label: string; tone: GatewaySignalTone }> = {
  critical: { label: "Critical", tone: "danger" },
  error: { label: "Error", tone: "danger" },
  warning: { label: "Warning", tone: "warning" },
  info: { label: "Info", tone: "info" },
};

export function auditSeverityCopy(severity: string): { label: string; tone: GatewaySignalTone } {
  return AUDIT_SEVERITY_COPY[severity] ?? { label: severity, tone: "neutral" };
}

export function ingressReachabilityCopy(reachability: string) {
  if (isGatewayIngressReachability(reachability)) {
    return INGRESS_REACHABILITY_COPY[reachability];
  }
  return {
    label: "Unknown",
    tone: "neutral" as GatewaySignalTone,
    detail: "Delivery status is unavailable.",
  };
}

function isGatewayIngressReachability(value: string): value is GatewayIngressReachability {
  return Object.prototype.hasOwnProperty.call(INGRESS_REACHABILITY_COPY, value);
}

export const DEVICE_ORIGIN_LABEL: Record<string, string> = {
  local: "This machine",
  private: "Private overlay",
  uds: "This machine",
};

export function deviceOriginLabel(origin: string): string {
  return DEVICE_ORIGIN_LABEL[origin] ?? origin;
}

export const DEVICE_ACTOR_LABEL: Record<string, string> = {
  operator_device: "Browser",
  cli_profile: "CLI profile",
};

export function deviceActorLabel(actorKind: string): string {
  return DEVICE_ACTOR_LABEL[actorKind] ?? actorKind;
}
