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
  "agy",
];

/** Collectors that read the provider CLI's own sign-in; the rest read a bound `api_key`. */
const NATIVE_CLI_COLLECTOR_IDS = new Set([
  "codex",
  "claude",
  "kimi-cli",
  "cursor",
  "gemini",
  "agy",
]);

export interface ProviderUsageFamilySettings {
  runtime_provider?: string;
  auth_mode?: string;
  credential_slots?: ReadonlyArray<{ name: string }>;
}

/**
 * The collector a provider's usage goes through, mirroring the daemon: its own
 * ID when it has a collector, else its `runtime_provider` when that collector
 * supports the provider's auth mode (bound-secret aliases also need an
 * `api_key` slot). `null` when the provider cannot collect.
 */
export function providerUsageCollectorFamily(
  id: string,
  settings: ProviderUsageFamilySettings | undefined
): string | null {
  if (PROVIDER_USAGE_COLLECTOR_IDS.includes(id)) return id;
  const runtime = settings?.runtime_provider?.trim() ?? "";
  if (!PROVIDER_USAGE_COLLECTOR_IDS.includes(runtime)) return null;
  const slots = settings?.credential_slots ?? [];
  const mode = settings?.auth_mode || (slots.length > 0 ? "bound_secret" : "native_cli");
  if (NATIVE_CLI_COLLECTOR_IDS.has(runtime)) return mode === "native_cli" ? runtime : null;
  return mode === "bound_secret" && slots.some(slot => slot.name === "api_key") ? runtime : null;
}

/** Collectors that read their CLI's login from the macOS Keychain and need consent first. */
export const PROVIDER_USAGE_KEYCHAIN_PROVIDERS: readonly string[] = ["claude", "agy"];

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

export function providerUsageKeychainAccess(
  config: ProviderUsageSettingsConfig,
  providerId: string
): boolean {
  return config.providers[providerId]?.keychain_access ?? false;
}

export function withProviderUsageKeychainAccess(
  config: ProviderUsageSettingsConfig,
  providerId: string,
  keychainAccess: boolean
): ProviderUsageSettingsConfig {
  return {
    ...config,
    providers: {
      ...config.providers,
      [providerId]: { ...config.providers[providerId], keychain_access: keychainAccess },
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
  return PROVIDER_USAGE_KEYCHAIN_PROVIDERS.every(
    id => providerUsageKeychainAccess(left, id) === providerUsageKeychainAccess(right, id)
  );
}
