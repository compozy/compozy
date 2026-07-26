import type { PillTone } from "@agh/ui";

const MODEL_AVAILABILITY_LABELS: Record<string, string> = {
  available_live: "live",
  available_stale: "stale",
  unavailable_live: "unavailable",
  unavailable_stale: "stale · unavailable",
  unknown: "unknown",
};

const MODEL_AVAILABILITY_TONES: Record<string, PillTone> = {
  available_live: "success",
  available_stale: "warning",
  unavailable_live: "danger",
  unavailable_stale: "warning",
  unknown: "neutral",
};

const MODEL_REFRESH_STATE_TONES: Record<string, PillTone> = {
  idle: "neutral",
  refreshing: "info",
  succeeded: "success",
  failed: "danger",
};

const PROVIDER_HEALTH_TONES: Record<string, PillTone> = {
  healthy: "success",
  unhealthy: "danger",
};

const PROVIDER_STATE_TONES: Record<string, PillTone> = {
  active: "success",
  error: "danger",
  registered: "info",
  enabled: "warning",
};

function modelAvailabilityLabel(state: string): string {
  return MODEL_AVAILABILITY_LABELS[state] ?? state;
}

function modelAvailabilityTone(state: string): PillTone {
  return MODEL_AVAILABILITY_TONES[state] ?? "neutral";
}

function modelRefreshStateTone(state: string): PillTone {
  return MODEL_REFRESH_STATE_TONES[state] ?? "neutral";
}

function providerHealthTone(health?: string): PillTone {
  return health ? (PROVIDER_HEALTH_TONES[health] ?? "neutral") : "neutral";
}

function providerStateTone(state?: string): PillTone {
  return state ? (PROVIDER_STATE_TONES[state] ?? "neutral") : "neutral";
}

export {
  modelAvailabilityLabel,
  modelAvailabilityTone,
  modelRefreshStateTone,
  providerHealthTone,
  providerStateTone,
};
