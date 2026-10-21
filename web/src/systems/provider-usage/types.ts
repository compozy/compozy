// Wire types for the experimental provider-usage surface (`_dx.md`).
//
// Aliases of the generated OpenAPI schemas. The daemon documents its enums as
// plain strings, so the fields this system branches on are narrowed to the
// closed sets `_dx.md` defines; every other field is the generated shape.

import type { components } from "@compozy/client";

type Schemas = components["schemas"];

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

/** Replaces the generated `string` fields named in `Narrow` with their closed sets. */
type Narrowed<Schema, Narrow> = Omit<Schema, keyof Narrow> & Narrow;

export type ProviderUsageAmount = Schemas["ProviderUsageAmount"];

export type ProviderUsageWindow = Narrowed<
  Schemas["ProviderUsageWindow"],
  { kind: ProviderUsageWindowKind }
>;

export type ProviderUsageBalance = Schemas["ProviderUsageBalance"];

export type ProviderUsageSpendCap = Schemas["ProviderUsageSpendCap"];

export type ProviderUsageSource = Narrowed<
  Schemas["ProviderUsageSource"],
  { kind: ProviderUsageSourceKind }
>;

export type ProviderUsageAccount = Schemas["ProviderUsageAccount"];

export type ProviderUsageError = Narrowed<
  Schemas["ProviderUsageError"],
  { action: ProviderUsageErrorAction }
>;

export type ProviderUsageTarget = Narrowed<
  Schemas["ProviderUsageTarget"],
  {
    status: ProviderUsageStatus;
    confidence: ProviderUsageConfidence;
    source: ProviderUsageSource | null;
    windows: ProviderUsageWindow[];
    error: ProviderUsageError | null;
  }
>;

export type ProviderUsageTraySignal = Narrowed<
  Schemas["ProviderUsageTraySignal"],
  { severity: ProviderUsageSeverity }
>;

export type ProviderUsageEnvelope = Narrowed<
  Schemas["ProviderUsageEnvelope"],
  { targets: ProviderUsageTarget[]; tray: ProviderUsageTraySignal | null }
>;

export type ProviderUsageRefreshRequest = Narrowed<
  Schemas["ProviderUsageRefreshRequest"],
  { intent?: ProviderUsageRefreshIntent }
>;

export type ProviderUsageStreamEvent = Schemas["ProviderUsageStreamEvent"];
