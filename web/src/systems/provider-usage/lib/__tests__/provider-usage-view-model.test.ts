// Suite: Provider usage view model
// Invariant: The daemon envelope maps to truthful presentation — severity bands, the binding
// window per row, native-unit money, client-derived reset text, status reasons with only the
// actions the host can honor, and the popover-open refresh set.
// Boundary IN: pure lib (format, severity, status, view-model) fed `_dx.md`-shaped fixtures.
// Boundary OUT: rendering (components suite) and request/stream wiring (hooks suite).
import { describe, expect, it } from "vitest";

import {
  PROVIDER_USAGE_FIXTURE_NOW as NOW,
  codexOkTarget,
  defaultUsageEnvelope,
  statusTargets,
  usageEnvelope,
  usageTarget,
  usageTray,
  usageWindow,
} from "../../mocks/fixtures";
import {
  formatAgo,
  formatAmount,
  formatPercent,
  formatPlan,
  formatResetsAt,
  formatWindowReset,
} from "../format";
import { selectTightestWindow, severityForPercent, toneForSeverity } from "../severity";
import { describeProviderUsageStatus } from "../status";
import {
  buildProviderUsageViewModel,
  formatTrayLabel,
  hasProviderUsageTargets,
  targetsNeedingOpenRefresh,
} from "../view-model";

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const iso = (offsetMs: number) => new Date(NOW + offsetMs).toISOString();

describe("severity", () => {
  it("Should band percent used into neutral, warning, and exhausted", () => {
    expect(severityForPercent(0)).toBe("neutral");
    expect(severityForPercent(79.9)).toBe("neutral");
    expect(severityForPercent(80)).toBe("warning");
    expect(severityForPercent(99.9)).toBe("warning");
    expect(severityForPercent(100)).toBe("exhausted");
    expect(severityForPercent(112)).toBe("exhausted");
    expect(toneForSeverity("exhausted")).toBe("destructive");
    expect(toneForSeverity("neutral")).toBe("neutral");
  });

  it("Should pick the highest known window and break ties on the earlier reset", () => {
    const tightest = selectTightestWindow([
      usageWindow({ id: "session", label: "Session", used_percent: 40, resets_at: iso(HOUR) }),
      usageWindow({ id: "late", label: "Late", used_percent: 88, resets_at: iso(3 * DAY) }),
      usageWindow({ id: "early", label: "Early", used_percent: 88, resets_at: iso(DAY) }),
      usageWindow({ id: "unknown", label: "Unknown", used_percent: null }),
    ]);
    expect(tightest?.id).toBe("early");
  });

  it("Should fall back to the first window when no percent is known", () => {
    const tightest = selectTightestWindow([
      usageWindow({ id: "a", label: "A", used_percent: null }),
      usageWindow({ id: "b", label: "B", used_percent: null }),
    ]);
    expect(tightest?.id).toBe("a");
    expect(selectTightestWindow([])).toBeNull();
  });
});

describe("format", () => {
  it("Should never round a sub-100 percent up to an exhausted-looking value", () => {
    expect(formatPercent(72.4)).toBe("72%");
    expect(formatPercent(99.7)).toBe("99%");
    expect(formatPercent(112)).toBe("112%");
  });

  it("Should keep native units and currencies", () => {
    expect(formatAmount({ value: "12.40", unit: "USD" })).toBe("$12.40");
    expect(formatAmount({ value: "120", unit: "credits" })).toBe("120 credits");
    expect(formatAmount({ value: "30", unit: "CNY" })).toBe("CN¥30.00");
    expect(formatAmount({ value: "n/a", unit: "tokens" })).toBe("n/a tokens");
  });

  it("Should derive reset text from the absolute reset time (UT-061)", () => {
    expect(formatResetsAt(iso(2 * DAY + 4 * HOUR), NOW)).toBe("resets in 2d 4h");
    expect(formatResetsAt(iso(3 * HOUR), NOW)).toBe("resets in 3h");
    expect(formatResetsAt(iso(2 * MINUTE), NOW)).toBe("resets in 2m");
    expect(formatResetsAt(iso(-1), NOW)).toBe("Resetting…");
    expect(formatResetsAt("2026-10-28T12:00:00Z", NOW)).toBe("Resets Oct 28");
    expect(formatResetsAt(null, NOW)).toBeNull();
  });

  it("Should show provider reset text verbatim only when no time exists (UT-059)", () => {
    expect(formatWindowReset({ resets_at: null, reset_text: "Resets monthly" }, NOW)).toBe(
      "Resets monthly"
    );
    expect(formatWindowReset({ resets_at: null, reset_text: "" }, NOW)).toBeNull();
    expect(formatWindowReset({ resets_at: iso(3 * HOUR), reset_text: "x" }, NOW)).toBe(
      "resets in 3h"
    );
  });

  it("Should describe age and plan for display", () => {
    expect(formatAgo(iso(-20_000), NOW)).toBe("just now");
    expect(formatAgo(iso(-18 * MINUTE), NOW)).toBe("18 min ago");
    expect(formatAgo(iso(-3 * HOUR), NOW)).toBe("3 hours ago");
    expect(formatPlan("max_20x")).toBe("Max 20x");
  });
});

describe("tray label", () => {
  it("Should carry the window, percent, and countdown (UT-050)", () => {
    expect(formatTrayLabel(usageTray({}, NOW), null, NOW)).toBe(
      "Codex · Weekly 72% used · resets in 2d 4h"
    );
  });

  it("Should say limit reached at or above 100 and how old stale data is", () => {
    const tray = usageTray({ used_percent: 112, severity: "exhausted", stale: true }, NOW);
    expect(formatTrayLabel(tray, iso(-25 * MINUTE), NOW)).toBe(
      "Codex · Weekly limit reached · resets in 2d 4h · updated 25 min ago"
    );
  });

  it("Should read Provider usage with no known window (UT-051)", () => {
    expect(formatTrayLabel(null, null, NOW)).toBe("Provider usage");
  });
});

describe("buildProviderUsageViewModel", () => {
  it("Should keep daemon order and claim the oldest measured time", () => {
    const viewModel = buildProviderUsageViewModel(defaultUsageEnvelope(NOW));
    expect(viewModel.rows.map(row => row.providerId)).toEqual(["claude", "codex"]);
    expect(viewModel.oldestMeasuredAt).toBe(iso(-2 * MINUTE));
    expect(viewModel.rows[0]?.tightest?.id).toBe("weekly_opus");
    expect(viewModel.rows[0]?.severity).toBe("warning");
    expect(viewModel.trayMeasuredAt).toBe(iso(-1 * MINUTE));
  });

  it("Should draw no window for rows without usage data", () => {
    const status = statusTargets(NOW);
    const viewModel = buildProviderUsageViewModel(
      usageEnvelope([status.unsupported, status.error], {}, NOW)
    );
    expect(viewModel.rows.every(row => row.tightest === null)).toBe(true);
    expect(viewModel.oldestMeasuredAt).toBeNull();
  });

  it("Should expose the tray control only while enabled with targets (UT-052)", () => {
    expect(hasProviderUsageTargets(null)).toBe(false);
    expect(
      hasProviderUsageTargets(
        buildProviderUsageViewModel(usageEnvelope([], { enabled: false }, NOW))
      )
    ).toBe(false);
    expect(hasProviderUsageTargets(buildProviderUsageViewModel(usageEnvelope([], {}, NOW)))).toBe(
      false
    );
    expect(hasProviderUsageTargets(buildProviderUsageViewModel(defaultUsageEnvelope(NOW)))).toBe(
      true
    );
  });

  it("Should carry the daemon's Keychain support into the view model", () => {
    const of = (keychain_supported: boolean) =>
      buildProviderUsageViewModel(usageEnvelope([], { keychain_supported }, NOW));
    expect(of(true).keychainSupported).toBe(true);
    expect(of(false).keychainSupported).toBe(false);
  });

  it("Should refresh on open only targets older than 60 s (UT-054)", () => {
    const fresh = { ...codexOkTarget(NOW), measured_at: iso(-30_000) };
    const old = usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      measured_at: iso(-90_000),
    });
    const status = statusTargets(NOW);
    const envelope = usageEnvelope([fresh, old, status.unsupported, status.refreshing], {}, NOW);
    expect(targetsNeedingOpenRefresh(envelope, NOW)).toEqual(["claude"]);
    const uncollected = { ...status.refreshing, refreshing: false };
    expect(targetsNeedingOpenRefresh(usageEnvelope([uncollected], {}, NOW), NOW)).toEqual([
      uncollected.provider_id,
    ]);
  });
});

describe("describeProviderUsageStatus (UT-063)", () => {
  const status = statusTargets(NOW);
  const mac = { now: NOW, isMacHost: true };
  const linux = { now: NOW, isMacHost: false };

  it.each([
    [
      "signed_out",
      status.codexAuthExpired,
      mac,
      "Signed out",
      "Signed out · Run Codex to refresh your login",
      null,
    ],
    ["needs_consent", status.needsConsent, mac, "Needs access", "Needs Keychain access", "Allow"],
    [
      "keychain_denied",
      status.keychainDenied,
      mac,
      "Access denied",
      "Keychain access denied",
      "Try again",
    ],
    [
      "keychain_read_timeout",
      {
        ...status.keychainDenied,
        error: {
          code: "keychain_read_timeout",
          message:
            "Background Keychain read timed out; an authorization prompt may have been required",
          action: "refresh",
          retry_after: null,
        },
      },
      mac,
      "Needs refresh",
      "Background Keychain read timed out; an authorization prompt may have been required",
      "Refresh",
    ],
    [
      "keychain_token_expired",
      status.keychainTokenExpired,
      mac,
      "Needs refresh",
      "Refresh to reload your Claude login",
      "Refresh",
    ],
    [
      "rate_limited",
      status.rateLimited,
      mac,
      "Rate limited",
      "Rate limited · retry in 4m",
      "Retry",
    ],
    ["error", status.error, mac, "Couldn't load", "Couldn't load usage", "Retry"],
    [
      "unsupported",
      status.unsupported,
      mac,
      "Unavailable",
      "Usage not available for this provider",
      null,
    ],
    ["no_plan", status.noPlan, mac, "Unavailable", "No coding plan on this key", "Open settings"],
    ["disabled", status.disabled, mac, "Turned off", "Turned off in settings", "Open settings"],
    ["stale", status.stale, mac, "Stale", "Stale · updated 18 min ago", "Retry"],
    [
      "needs_consent off macOS",
      status.needsConsent,
      linux,
      "Needs access",
      "Needs Keychain access",
      null,
    ],
    [
      "denied off macOS",
      status.keychainDenied,
      linux,
      "Access denied",
      "Keychain access denied",
      null,
    ],
  ] as const)("Should describe %s", (_name, target, options, label, text, actionLabel) => {
    const presentation = describeProviderUsageStatus(target, options);
    expect(presentation?.label).toBe(label);
    expect(presentation?.text).toBe(text);
    expect(presentation?.action?.label ?? null).toBe(actionLabel);
  });

  it("Should retry denied Keychain access without saving consent again", () => {
    expect(describeProviderUsageStatus(status.keychainDenied, mac)?.action?.kind).toBe("retry");
  });

  it("Should keep unsupported and disabled rows text only", () => {
    expect(describeProviderUsageStatus(status.unsupported, mac)?.tone).toBeNull();
    expect(describeProviderUsageStatus(status.disabled, mac)?.tone).toBeNull();
    expect(describeProviderUsageStatus(codexOkTarget(NOW), mac)).toBeNull();
  });

  it.each([status.needsConsent, status.keychainTokenExpired])(
    "Should route remote consent and credential reload to the host Mac",
    target => {
      const presentation = describeProviderUsageStatus(target, {
        ...mac,
        remoteClient: true,
      });
      expect(presentation?.note).toBe("Approve on the host Mac: open CompozyOS there and refresh");
    }
  );
});
