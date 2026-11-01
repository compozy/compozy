import type { ProviderUsageSeverity, ProviderUsageWindow } from "../types";

import { compareResetsAt } from "./format";

export const PROVIDER_USAGE_WARNING_PERCENT = 80;
export const PROVIDER_USAGE_EXHAUSTED_PERCENT = 100;

/** `Progress` tones the usage meters use; brand colors never tint a bar. */
export type ProviderUsageTone = "neutral" | "warning" | "destructive";

/** `< 80` neutral, `80–<100` warning, `≥ 100` exhausted (values above 100 included). */
export function severityForPercent(usedPercent: number): ProviderUsageSeverity {
  if (usedPercent >= PROVIDER_USAGE_EXHAUSTED_PERCENT) return "exhausted";
  if (usedPercent >= PROVIDER_USAGE_WARNING_PERCENT) return "warning";
  return "neutral";
}

const SEVERITY_TONE: Record<ProviderUsageSeverity, ProviderUsageTone> = {
  neutral: "neutral",
  warning: "warning",
  exhausted: "destructive",
};

export function toneForSeverity(severity: ProviderUsageSeverity): ProviderUsageTone {
  return SEVERITY_TONE[severity];
}

export function hasKnownPercent(
  window: ProviderUsageWindow
): window is ProviderUsageWindow & { used_percent: number } {
  return typeof window.used_percent === "number" && Number.isFinite(window.used_percent);
}

/**
 * The row's binding window: the highest known percent used, ties to the
 * earliest reset. With no known percent the first window still names the
 * limit (its percent renders as unknown); without windows there is none.
 */
export function selectTightestWindow(
  windows: readonly ProviderUsageWindow[]
): ProviderUsageWindow | null {
  let tightest: (ProviderUsageWindow & { used_percent: number }) | null = null;
  for (const window of windows) {
    if (!hasKnownPercent(window)) continue;
    if (
      tightest === null ||
      window.used_percent > tightest.used_percent ||
      (window.used_percent === tightest.used_percent &&
        compareResetsAt(window.resets_at, tightest.resets_at) < 0)
    ) {
      tightest = window;
    }
  }
  return tightest ?? windows[0] ?? null;
}
