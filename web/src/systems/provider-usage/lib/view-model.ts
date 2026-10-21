import type {
  ProviderUsageEnvelope,
  ProviderUsageSeverity,
  ProviderUsageTarget,
  ProviderUsageTraySignal,
  ProviderUsageWindow,
} from "../types";

import { formatAgo, formatPercent, formatResetsAt, isOlderThan } from "./format";
import { hasKnownPercent, selectTightestWindow, severityForPercent } from "./severity";
import { showsUsageData } from "./status";

export interface ProviderUsageRowModel {
  providerId: string;
  target: ProviderUsageTarget;
  /** The binding window when the row has usage data to draw. */
  tightest: ProviderUsageWindow | null;
  /** Severity of the tightest known percent; `null` when unknown. */
  severity: ProviderUsageSeverity | null;
  stale: boolean;
}

export interface ProviderUsageViewModel {
  profileId: string;
  revision: number;
  enabled: boolean;
  tray: ProviderUsageTraySignal | null;
  /** When the tray window's data was measured, for the stale tooltip. */
  trayMeasuredAt: string | null;
  /** Ordered as the daemon returns them (provider catalog order). */
  rows: ProviderUsageRowModel[];
  /** Oldest `measured_at` among displayed rows: what "Updated N min ago" claims. */
  oldestMeasuredAt: string | null;
}

export function toProviderUsageRow(target: ProviderUsageTarget): ProviderUsageRowModel {
  const hasData = showsUsageData(target);
  const tightest = hasData ? selectTightestWindow(target.windows) : null;
  return {
    providerId: target.provider_id,
    target,
    tightest,
    severity:
      tightest && hasKnownPercent(tightest) ? severityForPercent(tightest.used_percent) : null,
    stale: target.status === "stale",
  };
}

function oldestMeasuredAt(targets: readonly ProviderUsageTarget[]): string | null {
  let oldest: { iso: string; time: number } | null = null;
  for (const target of targets) {
    if (!showsUsageData(target) || !target.measured_at) continue;
    const time = Date.parse(target.measured_at);
    if (!Number.isFinite(time)) continue;
    if (oldest === null || time < oldest.time) oldest = { iso: target.measured_at, time };
  }
  return oldest?.iso ?? null;
}

export function buildProviderUsageViewModel(
  envelope: ProviderUsageEnvelope
): ProviderUsageViewModel {
  const trayTarget = envelope.tray
    ? envelope.targets.find(target => target.provider_id === envelope.tray?.provider_id)
    : undefined;
  return {
    profileId: envelope.profile_id,
    revision: envelope.revision,
    enabled: envelope.enabled,
    tray: envelope.tray,
    trayMeasuredAt: trayTarget?.measured_at ?? null,
    rows: envelope.targets.map(toProviderUsageRow),
    oldestMeasuredAt: oldestMeasuredAt(envelope.targets),
  };
}

/** The tray control exists only while the feature is on and the profile has a target. */
export function hasProviderUsageTargets(viewModel: ProviderUsageViewModel | null): boolean {
  return viewModel !== null && viewModel.enabled && viewModel.rows.length > 0;
}

export const PROVIDER_USAGE_TITLE = "Provider usage";

const KEYCHAIN_CONSENT_CODES: ReadonlySet<string> = new Set([
  "keychain_consent_required",
  "keychain_denied",
  "keychain_token_expired",
]);

/**
 * The daemon reports a Keychain consent state, which it does only when it runs
 * on macOS: host evidence that does not depend on this client's own platform.
 */
export function reportsKeychainConsent(viewModel: ProviderUsageViewModel | null): boolean {
  return (
    viewModel?.rows.some(row => KEYCHAIN_CONSENT_CODES.has(row.target.error?.code ?? "")) ?? false
  );
}

/**
 * The tray's accessible name and tooltip: "Codex · Weekly 72% used · resets
 * in 2d 4h". At or above 100 the label says "limit reached" (the exact number
 * lives in the detail); stale data says how old it is.
 */
export function formatTrayLabel(
  tray: ProviderUsageTraySignal | null,
  measuredAt: string | null,
  now: number
): string {
  if (tray === null) return PROVIDER_USAGE_TITLE;
  const usage =
    tray.severity === "exhausted" ? "limit reached" : `${formatPercent(tray.used_percent)} used`;
  const parts = [`${tray.label} ${usage}`];
  const reset = formatResetsAt(tray.resets_at, now);
  if (reset) parts.push(reset);
  if (tray.stale) {
    const ago = formatAgo(measuredAt, now);
    parts.push(ago ? `updated ${ago}` : "stale");
  }
  return parts.join(" · ");
}

export const PROVIDER_USAGE_OPEN_MAX_AGE_SECONDS = 60;

/**
 * Targets a popover open should refresh: data older than the open threshold.
 * Rows with nothing to collect (unsupported, disabled) or already collecting
 * are left alone.
 */
export function targetsNeedingOpenRefresh(
  envelope: ProviderUsageEnvelope,
  now: number,
  maxAgeSeconds = PROVIDER_USAGE_OPEN_MAX_AGE_SECONDS
): string[] {
  const ids: string[] = [];
  for (const target of envelope.targets) {
    if (
      target.status !== "unsupported" &&
      target.status !== "disabled" &&
      target.status !== "refreshing" &&
      !target.refreshing &&
      isOlderThan(target.measured_at, now, maxAgeSeconds * 1_000)
    ) {
      ids.push(target.provider_id);
    }
  }
  return ids;
}
