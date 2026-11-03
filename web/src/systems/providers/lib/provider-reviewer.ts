import type { ProviderSummary } from "../types";

/** Whether a provider declares an AI reviewer that the `auto` level can select. */
export type ProviderReviewer = "available" | "unavailable" | "unknown";

/**
 * The daemon always sends `reviewer_mode`; an empty string means the provider has no reviewer.
 * An absent field only comes from an older daemon, so it stays `unknown` and never warns.
 */
export function providerReviewer(summary: ProviderSummary | undefined): ProviderReviewer {
  const mode = summary?.reviewer_mode;
  if (mode === undefined) return "unknown";
  return mode.trim() ? "available" : "unavailable";
}
