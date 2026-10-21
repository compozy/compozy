// Wire types for the experimental provider-usage surface (`_dx.md`).
//
// Hand-written mirrors of the daemon's OpenAPI schemas, under the schema names,
// until the generated client carries them. Swap each one for its
// `components["schemas"]["ProviderUsage…"]` alias in this file only.

export type ProviderUsageStatus =
  | "ok"
  | "stale"
  | "refreshing"
  | "signed_out"
  | "needs_consent"
  | "rate_limited"
  | "error"
  | "unsupported"
  | "disabled";

export type ProviderUsageSeverity = "neutral" | "warning" | "exhausted";

export type ProviderUsageSourceKind = "cli_rpc" | "oauth_http" | "api_key_http" | "acp_observed";

export type ProviderUsageConfidence = "exact" | "percent_only" | "estimated";

export type ProviderUsageWindowKind =
  | "session"
  | "daily"
  | "weekly"
  | "monthly"
  | "model"
  | "other";

export type ProviderUsageErrorAction =
  | "sign_in"
  | "allow_keychain"
  | "refresh"
  | "retry"
  | "configure"
  | "";

export type ProviderUsageRefreshIntent = "background" | "explicit";

export interface ProviderUsageAmount {
  /** Decimal string; never rounded through a float. */
  value: string;
  /** "USD", "CNY", "credits", "requests", "tokens". */
  unit: string;
}

export interface ProviderUsageWindow {
  id: string;
  label: string;
  kind: ProviderUsageWindowKind;
  duration_seconds: number | null;
  /** Percent used in `[0, ∞)`; `null` is unknown, never 0. */
  used_percent: number | null;
  used: ProviderUsageAmount | null;
  limit: ProviderUsageAmount | null;
  resets_at: string | null;
  /** Provider text when no timestamp exists. */
  reset_text: string;
}

export interface ProviderUsageBalance {
  id: string;
  label: string;
  remaining: ProviderUsageAmount;
  expires_at: string | null;
}

export interface ProviderUsageSpendCap {
  id: string;
  label: string;
  used: ProviderUsageAmount;
  /** `null` = no cap reported, which is not "unlimited". */
  limit: ProviderUsageAmount | null;
  used_percent: number | null;
  resets_at: string | null;
  enabled: boolean | null;
}

export interface ProviderUsageSource {
  kind: ProviderUsageSourceKind;
  label: string;
}

export interface ProviderUsageAccount {
  label: string;
  plan: string;
}

export interface ProviderUsageError {
  code: string;
  message: string;
  action: ProviderUsageErrorAction;
  retry_after: string | null;
}

export interface ProviderUsageTarget {
  provider_id: string;
  provider_name: string;
  status: ProviderUsageStatus;
  source: ProviderUsageSource | null;
  account: ProviderUsageAccount | null;
  windows: ProviderUsageWindow[];
  balances: ProviderUsageBalance[];
  spend_caps: ProviderUsageSpendCap[];
  confidence: ProviderUsageConfidence;
  measured_at: string | null;
  checked_at: string | null;
  next_refresh_at: string | null;
  refreshing: boolean;
  error: ProviderUsageError | null;
}

export interface ProviderUsageTraySignal {
  provider_id: string;
  window_id: string;
  /** "Codex · Weekly". */
  label: string;
  used_percent: number;
  resets_at: string | null;
  severity: ProviderUsageSeverity;
  stale: boolean;
}

export interface ProviderUsageEnvelope {
  profile_id: string;
  revision: number;
  generated_at: string;
  enabled: boolean;
  tray: ProviderUsageTraySignal | null;
  targets: ProviderUsageTarget[];
}

export interface ProviderUsageRefreshRequest {
  profile?: string;
  provider_ids?: string[];
  max_age_seconds?: number;
  wait_ms?: number;
  intent?: ProviderUsageRefreshIntent;
}

export interface ProviderUsageStreamEvent {
  revision: number;
  boot_id: string;
}
