"use client";

import { useQuery } from "@tanstack/react-query";

import { ProviderUsageApiError } from "../adapters/provider-usage-api";
import { providerUsageOptions } from "../lib/query-options";
import { buildProviderUsageViewModel, type ProviderUsageViewModel } from "../lib/view-model";

export interface UseProviderUsageOptions {
  enabled?: boolean;
}

export interface ProviderUsageState {
  /** `null` until the first envelope arrives. Kept across later read failures. */
  viewModel: ProviderUsageViewModel | null;
  /**
   * No envelope yet and no failure either: the first read is in flight, held back, or paused. A
   * refresh behind a cached envelope is not loading.
   */
  loading: boolean;
  /**
   * The daemon cannot be reached. Cached rows stay on screen under a banner
   * and refresh actions are withdrawn until it answers again.
   */
  daemonUnavailable: boolean;
  /** A read failure other than reachability (unknown profile, …). */
  errorMessage: string | null;
}

/** The one view model behind the tray, the popover, and the Usage app. */
export function useProviderUsage(
  profile: string,
  { enabled = true }: UseProviderUsageOptions = {}
): ProviderUsageState {
  const scoped = profile.trim();
  const query = useQuery({
    ...providerUsageOptions(scoped),
    enabled: enabled && scoped !== "",
    select: buildProviderUsageViewModel,
  });
  const error = query.error;
  const daemonUnavailable = error instanceof ProviderUsageApiError && error.unreachable;
  return {
    viewModel: query.data ?? null,
    loading: query.isPending,
    daemonUnavailable,
    errorMessage: error instanceof Error && !daemonUnavailable ? error.message : null,
  };
}
