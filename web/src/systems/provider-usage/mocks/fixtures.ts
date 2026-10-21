import type {
  ProviderUsageEnvelope,
  ProviderUsageError,
  ProviderUsageTarget,
  ProviderUsageTraySignal,
  ProviderUsageWindow,
} from "../types";

// Fixtures are built relative to a clock so countdowns read the same in a
// story rendered today and in a test pinned to a fake time. Shapes follow
// `_dx.md`; no value is captured from a real account.

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

export const PROVIDER_USAGE_FIXTURE_NOW = Date.parse("2026-10-20T22:10:00Z");

function at(now: number, offsetMs: number): string {
  return new Date(now + offsetMs).toISOString();
}

export function usageWindow(
  overrides: Partial<ProviderUsageWindow> & Pick<ProviderUsageWindow, "id" | "label">
): ProviderUsageWindow {
  return {
    kind: "other",
    duration_seconds: null,
    used_percent: null,
    used: null,
    limit: null,
    resets_at: null,
    reset_text: "",
    ...overrides,
  };
}

export function usageTarget(
  overrides: Partial<ProviderUsageTarget> &
    Pick<ProviderUsageTarget, "provider_id" | "provider_name">
): ProviderUsageTarget {
  return {
    status: "ok",
    source: null,
    account: null,
    windows: [],
    balances: [],
    spend_caps: [],
    confidence: "percent_only",
    measured_at: null,
    checked_at: null,
    next_refresh_at: null,
    refreshing: false,
    error: null,
    ...overrides,
  };
}

function usageError(
  code: string,
  message: string,
  action: ProviderUsageError["action"],
  retryAfter: string | null = null
): ProviderUsageError {
  return { code, message, action, retry_after: retryAfter };
}

export function codexOkTarget(now = PROVIDER_USAGE_FIXTURE_NOW): ProviderUsageTarget {
  return usageTarget({
    provider_id: "codex",
    provider_name: "Codex",
    source: { kind: "cli_rpc", label: "Codex app-server" },
    account: { label: "pedro@example.com", plan: "pro" },
    windows: [
      usageWindow({
        id: "session",
        label: "Session",
        kind: "session",
        duration_seconds: 18_000,
        used_percent: 23,
        resets_at: at(now, 3 * HOUR),
      }),
      usageWindow({
        id: "weekly",
        label: "Weekly",
        kind: "weekly",
        duration_seconds: 604_800,
        used_percent: 72,
        resets_at: at(now, 2 * DAY + 4 * HOUR),
      }),
    ],
    balances: [
      {
        id: "credits",
        label: "Credits",
        remaining: { value: "120", unit: "credits" },
        expires_at: null,
      },
    ],
    measured_at: at(now, -2 * MINUTE),
    checked_at: at(now, -2 * MINUTE),
    next_refresh_at: at(now, 3 * MINUTE),
  });
}

export function claudeOkTarget(now = PROVIDER_USAGE_FIXTURE_NOW): ProviderUsageTarget {
  return usageTarget({
    provider_id: "claude",
    provider_name: "Claude",
    source: { kind: "oauth_http", label: "Claude Code OAuth" },
    account: { label: "pedro@example.com", plan: "max_20x" },
    confidence: "exact",
    windows: [
      usageWindow({
        id: "session",
        label: "Session",
        kind: "session",
        used_percent: 34,
        resets_at: at(now, 2 * HOUR + 12 * MINUTE),
      }),
      usageWindow({
        id: "weekly",
        label: "Weekly",
        kind: "weekly",
        used_percent: 61,
        resets_at: at(now, 3 * DAY + 2 * HOUR),
      }),
      usageWindow({
        id: "weekly_opus",
        label: "Weekly · Opus",
        kind: "model",
        used_percent: 88,
        resets_at: at(now, 3 * DAY + 2 * HOUR),
      }),
    ],
    spend_caps: [
      {
        id: "extra_usage",
        label: "Extra usage",
        used: { value: "12.40", unit: "USD" },
        limit: { value: "50.00", unit: "USD" },
        used_percent: 24.8,
        resets_at: null,
        enabled: true,
      },
    ],
    measured_at: at(now, -1 * MINUTE),
    checked_at: at(now, -1 * MINUTE),
  });
}

/** One target per row status and error code in the `_dx.md` table. */
export function statusTargets(now = PROVIDER_USAGE_FIXTURE_NOW) {
  return {
    signedOut: usageTarget({
      provider_id: "cursor",
      provider_name: "Cursor",
      status: "signed_out",
      error: usageError("auth_missing", "Sign in with cursor-agent to read usage", "sign_in"),
    }),
    codexAuthExpired: usageTarget({
      provider_id: "codex",
      provider_name: "Codex",
      status: "signed_out",
      error: usageError("auth_expired", "Run Codex to refresh your login", "sign_in"),
    }),
    claudeAuthScope: usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      status: "signed_out",
      error: usageError(
        "auth_scope",
        "This Claude login can't read usage. Sign in with claude to enable it.",
        "sign_in"
      ),
    }),
    needsConsent: usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      status: "needs_consent",
      error: usageError("keychain_consent_required", "Needs Keychain access", "allow_keychain"),
    }),
    keychainDenied: usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      status: "needs_consent",
      error: usageError("keychain_denied", "Keychain access denied", "allow_keychain"),
    }),
    keychainTokenExpired: usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      status: "needs_consent",
      error: usageError("keychain_token_expired", "Refresh to reload your Claude login", "refresh"),
    }),
    rateLimited: usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      status: "rate_limited",
      checked_at: at(now, -1 * MINUTE),
      error: usageError(
        "provider_rate_limited",
        "Rate limited by Claude · retry after 22:15",
        "retry",
        at(now, 4 * MINUTE)
      ),
    }),
    error: usageTarget({
      provider_id: "cursor",
      provider_name: "Cursor",
      status: "error",
      checked_at: at(now, -1 * MINUTE),
      error: usageError("parse_error", "Unexpected response from Cursor", "retry"),
    }),
    unsupported: usageTarget({
      provider_id: "pi",
      provider_name: "Pi",
      status: "unsupported",
      error: usageError("no_collector", "Usage not available for this provider", ""),
    }),
    noPlan: usageTarget({
      provider_id: "zai",
      provider_name: "Z.ai",
      status: "unsupported",
      error: usageError("no_plan", "No coding plan on this key", "configure"),
    }),
    disabled: usageTarget({
      provider_id: "gemini",
      provider_name: "Gemini",
      status: "disabled",
      error: usageError("disabled", "Turned off in settings", "configure"),
    }),
    refreshing: usageTarget({
      provider_id: "codex",
      provider_name: "Codex",
      status: "refreshing",
      refreshing: true,
    }),
    stale: {
      ...codexOkTarget(now),
      status: "stale" as const,
      measured_at: at(now, -18 * MINUTE),
      checked_at: at(now, -1 * MINUTE),
      error: usageError("network_error", "Couldn't reach chatgpt.com", "retry"),
    },
  } satisfies Record<string, ProviderUsageTarget>;
}

/** Edge windows and money from US-003/US-004. */
export function edgeCaseTarget(now = PROVIDER_USAGE_FIXTURE_NOW): ProviderUsageTarget {
  return usageTarget({
    provider_id: "openrouter",
    provider_name: "OpenRouter",
    source: { kind: "api_key_http", label: "OpenRouter API key" },
    confidence: "estimated",
    windows: [
      usageWindow({ id: "daily", label: "Daily", kind: "daily", resets_at: at(now, 3 * HOUR) }),
      usageWindow({
        id: "monthly",
        label: "Monthly",
        kind: "monthly",
        used_percent: 41,
        resets_at: at(now, 8 * DAY),
      }),
      usageWindow({
        id: "weekly",
        label: "Weekly",
        kind: "weekly",
        used_percent: 57,
        resets_at: at(now, -2 * MINUTE),
      }),
      usageWindow({
        id: "plan",
        label: "Plan",
        used_percent: 112,
        reset_text: "Resets with your next billing cycle",
      }),
    ],
    balances: [
      {
        id: "prepaid",
        label: "Prepaid",
        remaining: { value: "8.10", unit: "USD" },
        expires_at: null,
      },
      { id: "promo", label: "Promo", remaining: { value: "30", unit: "CNY" }, expires_at: null },
    ],
    spend_caps: [
      {
        id: "key_limit",
        label: "Key spend",
        used: { value: "8.10", unit: "USD" },
        limit: null,
        used_percent: null,
        resets_at: null,
        enabled: null,
      },
    ],
    measured_at: at(now, -4 * MINUTE),
  });
}

export function usageTray(
  overrides: Partial<ProviderUsageTraySignal> = {},
  now = PROVIDER_USAGE_FIXTURE_NOW
): ProviderUsageTraySignal {
  return {
    provider_id: "codex",
    window_id: "weekly",
    label: "Codex · Weekly",
    used_percent: 72,
    resets_at: at(now, 2 * DAY + 4 * HOUR),
    severity: "neutral",
    stale: false,
    ...overrides,
  };
}

export function usageEnvelope(
  targets: ProviderUsageTarget[],
  overrides: Partial<ProviderUsageEnvelope> = {},
  now = PROVIDER_USAGE_FIXTURE_NOW
): ProviderUsageEnvelope {
  return {
    profile_id: "default",
    revision: 14,
    generated_at: at(now, 0),
    enabled: true,
    tray: null,
    targets,
    ...overrides,
  };
}

/** The golden path: Claude and Codex both measured, Claude Opus weekly binding. */
export function defaultUsageEnvelope(now = PROVIDER_USAGE_FIXTURE_NOW): ProviderUsageEnvelope {
  return usageEnvelope(
    [claudeOkTarget(now), codexOkTarget(now)],
    {
      tray: usageTray(
        {
          provider_id: "claude",
          window_id: "weekly_opus",
          label: "Claude · Weekly · Opus",
          used_percent: 88,
          resets_at: at(now, 3 * DAY + 2 * HOUR),
          severity: "warning",
        },
        now
      ),
    },
    now
  );
}
