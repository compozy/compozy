import type { SettingsProviderUsageSection } from "../types";
import { parsePositiveDurationMilliseconds } from "./terminal-settings-duration";

export type ProviderUsageSettingsConfig = SettingsProviderUsageSection["config"];

export interface ProviderUsageIntervalOption {
  value: string;
  label: string;
}

/** The refresh cadences the section offers; the daemon accepts 1m–1h. */
export const PROVIDER_USAGE_INTERVAL_OPTIONS: readonly ProviderUsageIntervalOption[] = [
  { value: "1m", label: "Every minute" },
  { value: "2m", label: "Every 2 minutes" },
  { value: "5m", label: "Every 5 minutes" },
  { value: "10m", label: "Every 10 minutes" },
  { value: "15m", label: "Every 15 minutes" },
  { value: "30m", label: "Every 30 minutes" },
  { value: "1h", label: "Every hour" },
];

/** Canonical provider IDs with a v1 usage collector, in provider catalog order. */
export const PROVIDER_USAGE_COLLECTOR_IDS: readonly string[] = [
  "codex",
  "claude",
  "openrouter",
  "vercel-ai-gateway",
  "zai",
  "moonshot",
  "kimi-cli",
  "cursor",
  "gemini",
];

export const PROVIDER_USAGE_KEYCHAIN_PROVIDER = "claude";

/**
 * The option a stored interval matches. The daemon echoes Go durations
 * ("5m0s"), so values compare by length of time, not by spelling; a value set
 * elsewhere that is not on the list keeps its own spelling.
 */
export function providerUsageIntervalValue(interval: string): string {
  const millis = parsePositiveDurationMilliseconds(interval);
  const match = PROVIDER_USAGE_INTERVAL_OPTIONS.find(
    option => parsePositiveDurationMilliseconds(option.value) === millis
  );
  return match?.value ?? interval;
}

/** A provider collects unless the config turns it off. */
export function providerUsageProviderEnabled(
  config: ProviderUsageSettingsConfig,
  providerId: string
): boolean {
  return config.providers[providerId]?.enabled ?? true;
}

export function withProviderUsageProviderEnabled(
  config: ProviderUsageSettingsConfig,
  providerId: string,
  enabled: boolean
): ProviderUsageSettingsConfig {
  return {
    ...config,
    providers: { ...config.providers, [providerId]: { ...config.providers[providerId], enabled } },
  };
}

export function providerUsageKeychainAccess(config: ProviderUsageSettingsConfig): boolean {
  return config.providers[PROVIDER_USAGE_KEYCHAIN_PROVIDER]?.keychain_access ?? false;
}

export function withProviderUsageKeychainAccess(
  config: ProviderUsageSettingsConfig,
  keychainAccess: boolean
): ProviderUsageSettingsConfig {
  const id = PROVIDER_USAGE_KEYCHAIN_PROVIDER;
  return {
    ...config,
    providers: {
      ...config.providers,
      [id]: { ...config.providers[id], keychain_access: keychainAccess },
    },
  };
}

/** Compares what a save would change, ignoring duration spelling and default entries. */
export function sameProviderUsageConfig(
  left: ProviderUsageSettingsConfig,
  right: ProviderUsageSettingsConfig
): boolean {
  if (left.enabled !== right.enabled) return false;
  if (
    providerUsageIntervalValue(left.refresh_interval) !==
    providerUsageIntervalValue(right.refresh_interval)
  ) {
    return false;
  }
  const ids = new Set([...Object.keys(left.providers), ...Object.keys(right.providers)]);
  for (const id of ids) {
    if (providerUsageProviderEnabled(left, id) !== providerUsageProviderEnabled(right, id)) {
      return false;
    }
  }
  return providerUsageKeychainAccess(left) === providerUsageKeychainAccess(right);
}
