import type { ProviderUsageTarget } from "../types";

import { formatAgo, formatWaitUntil } from "./format";

/** Row actions the status line can offer; every one runs with explicit intent. */
export type ProviderUsageRowAction = "allow_keychain" | "retry" | "refresh";

export type ProviderUsageStatusTone = "warning" | "danger" | "faint";

export interface ProviderUsageStatusAction {
  kind: ProviderUsageRowAction;
  label: string;
}

export interface ProviderUsageStatusPresentation {
  /** `null` = text only (unsupported, disabled): no dot, nothing to fix here. */
  tone: ProviderUsageStatusTone | null;
  /** `ring` marks stale and loading rows by shape, not hue. */
  dotVariant: "solid" | "ring";
  text: string;
  action: ProviderUsageStatusAction | null;
  /** Extra guidance under the line (remote consent). */
  note: string | null;
}

export interface DescribeStatusOptions {
  now: number;
  /** The daemon runs on macOS: Keychain consent can be granted at all. */
  isMacHost: boolean;
  /** This client reaches the daemon remotely (gateway): consent prompts happen on the host. */
  remoteClient?: boolean;
}

export const UNSUPPORTED_TEXT = "Usage not available for this provider";
export const REMOTE_CONSENT_NOTE = "Approve on the host Mac: open CompozyOS there and refresh";

const RETRY: ProviderUsageStatusAction = { kind: "retry", label: "Retry" };
const REFRESH: ProviderUsageStatusAction = { kind: "refresh", label: "Refresh" };

function presentation(
  tone: ProviderUsageStatusTone | null,
  text: string,
  action: ProviderUsageStatusAction | null = null,
  dotVariant: "solid" | "ring" = "solid",
  note: string | null = null
): ProviderUsageStatusPresentation {
  return { tone, dotVariant, text, action, note };
}

function messageOr(target: ProviderUsageTarget, fallback: string): string {
  const message = target.error?.message.trim();
  return message ? message : fallback;
}

function describeNeedsConsent(
  target: ProviderUsageTarget,
  options: DescribeStatusOptions
): ProviderUsageStatusPresentation {
  const code = target.error?.code;
  if (code === "keychain_token_expired") {
    return presentation("warning", "Refresh to reload your Claude login", REFRESH);
  }
  const denied = code === "keychain_denied";
  const text = denied ? "Keychain access denied" : messageOr(target, "Needs Keychain access");
  if (!options.isMacHost) return presentation("warning", text);
  const action: ProviderUsageStatusAction = {
    kind: "allow_keychain",
    label: denied ? "Try again" : "Allow",
  };
  return presentation(
    "warning",
    text,
    action,
    "solid",
    options.remoteClient ? REMOTE_CONSENT_NOTE : null
  );
}

function describeRateLimited(
  target: ProviderUsageTarget,
  now: number
): ProviderUsageStatusPresentation {
  const wait = formatWaitUntil(target.error?.retry_after ?? null, now);
  const text = wait ? `Rate limited · retry in ${wait}` : messageOr(target, "Rate limited");
  return presentation("warning", text, RETRY);
}

function describeStale(target: ProviderUsageTarget, now: number): ProviderUsageStatusPresentation {
  const ago = formatAgo(target.measured_at, now);
  const text = ago ? `Stale · updated ${ago}` : "Stale";
  const action = target.error?.action === "refresh" ? REFRESH : RETRY;
  return presentation("faint", text, action, "ring");
}

/**
 * Status → reason and next step, per the `_dx.md` per-target table. `ok` rows
 * need no status line and return `null`. Sign-in and configuration happen
 * outside this surface, so those rows carry text and no button.
 */
export function describeProviderUsageStatus(
  target: ProviderUsageTarget,
  options: DescribeStatusOptions
): ProviderUsageStatusPresentation | null {
  switch (target.status) {
    case "ok":
      return null;
    case "refreshing":
      return presentation("faint", "Loading…", null, "ring");
    case "stale":
      return describeStale(target, options.now);
    case "signed_out": {
      const message = target.error?.message.trim();
      return presentation("faint", message ? `Signed out · ${message}` : "Signed out");
    }
    case "needs_consent":
      return describeNeedsConsent(target, options);
    case "rate_limited":
      return describeRateLimited(target, options.now);
    case "error":
      return presentation("danger", "Couldn't load usage", RETRY);
    case "unsupported":
      return presentation(null, messageOr(target, UNSUPPORTED_TEXT));
    case "disabled":
      return presentation(null, messageOr(target, "Turned off in settings"));
  }
}

/** Statuses whose windows carry numbers worth drawing. */
export function showsUsageData(target: ProviderUsageTarget): boolean {
  return target.status === "ok" || target.status === "stale";
}
