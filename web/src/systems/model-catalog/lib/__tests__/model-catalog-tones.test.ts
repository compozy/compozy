import { describe, expect, it } from "vitest";

import {
  modelAvailabilityLabel,
  modelAvailabilityTone,
  modelRefreshStateTone,
  providerHealthTone,
  providerStateTone,
} from "../model-catalog-tones";

describe("model-catalog tones", () => {
  it.each([
    ["available_live", "success", "live"],
    ["available_stale", "warning", "stale"],
    ["unavailable_live", "danger", "unavailable"],
    ["unavailable_stale", "warning", "stale · unavailable"],
    ["unknown", "neutral", "unknown"],
  ] as const)("Should map availability %s to %s and %s", (state, tone, label) => {
    expect(modelAvailabilityTone(state)).toBe(tone);
    expect(modelAvailabilityLabel(state)).toBe(label);
  });

  it.each([
    ["idle", "neutral"],
    ["refreshing", "info"],
    ["succeeded", "success"],
    ["failed", "danger"],
    ["new-state", "neutral"],
  ] as const)("Should map refresh state %s to %s", (state, tone) => {
    expect(modelRefreshStateTone(state)).toBe(tone);
  });

  it.each([
    ["healthy", "success"],
    ["unhealthy", "danger"],
    ["unknown", "neutral"],
    [undefined, "neutral"],
  ] as const)("Should map provider health %s to %s", (health, tone) => {
    expect(providerHealthTone(health)).toBe(tone);
  });

  it.each([
    ["active", "success"],
    ["error", "danger"],
    ["registered", "info"],
    ["enabled", "warning"],
    ["unknown", "neutral"],
    [undefined, "neutral"],
  ] as const)("Should map provider state %s to %s", (state, tone) => {
    expect(providerStateTone(state)).toBe(tone);
  });
});
