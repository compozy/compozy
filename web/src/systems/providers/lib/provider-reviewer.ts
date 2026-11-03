import type { ProviderSummary } from "../types";

/** Whether a provider declares an AI reviewer that the `auto` level can select. */
export type ProviderReviewer = "available" | "unavailable" | "unknown";

/** `reviewer_mode` arrives from the daemon's resolved provider config; empty means none. */
type ProviderWithReviewer = ProviderSummary & { reviewer_mode?: string | null };

/** Absent data stays `unknown`, so the editor never warns without provider evidence. */
export function providerReviewer(summary: ProviderSummary | undefined): ProviderReviewer {
  if (!summary) return "unknown";
  const mode = (summary as ProviderWithReviewer).reviewer_mode;
  if (mode === undefined || mode === null) return "unknown";
  return mode.trim() ? "available" : "unavailable";
}
