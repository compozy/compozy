import type {
  AccountDeviceFixture,
  AccountPlan,
  AccountProvider,
  AccountSessionSource,
  AccountStateFixture,
  AccountTokenFixture,
  ApprovalState,
  DeviceKind,
  PendingApprovalFixture,
  RemoteScope,
} from "@/systems/computers/mocks/remote-computers-types";

import { AccountApiError } from "../adapters/account-api-error";
import type {
  AccountApprovalPayload,
  AccountDevicePayload,
  AccountSignInNoticePayload,
  AccountStatusPayload,
  AccountTokenPayload,
} from "../adapters/account-api";

export function formatRelativeTimeLabel(iso: string | null | undefined): string {
  if (!iso) return "never";
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return iso;
  const diffSeconds = Math.max(0, Math.floor((Date.now() - parsed) / 1000));
  if (diffSeconds < 60) return "now";
  const minutes = Math.floor(diffSeconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function isAccountCloudUnreachableError(error: unknown): boolean {
  if (error instanceof AccountApiError) {
    return error.code === "account_cloud_unreachable" || error.status === 503;
  }
  return false;
}

function normalizeProvider(provider: string | undefined): AccountProvider | null {
  if (provider === "github") return "github";
  if (provider === "google") return "google";
  return null;
}

function normalizeSource(source: string | null | undefined): AccountSessionSource {
  if (source === "link" || source === "host_link") return "host_link";
  if (source === "token" || source === "account_token") return "account_token";
  return "session";
}

function normalizePlan(plan: string | undefined): AccountPlan {
  return plan === "pro" ? "pro" : "free";
}

function normalizeDeviceKind(kind: string | undefined): DeviceKind {
  if (kind === "browser" || kind === "cli" || kind === "desktop") return kind;
  return "desktop";
}

function normalizeApprovalState(state: string): ApprovalState {
  if (
    state === "pending" ||
    state === "delivering" ||
    state === "approved" ||
    state === "denied" ||
    state === "expired"
  ) {
    return state;
  }
  return "pending";
}

function inferApprovalDeviceKind(deviceName: string): DeviceKind {
  const lower = deviceName.toLowerCase();
  if (lower.includes("iphone") || lower.includes("ipad") || lower.includes("browser")) {
    return "browser";
  }
  if (lower.includes("cli")) return "cli";
  return "desktop";
}

/**
 * Builds a clean signed-out status snapshot preserving only installation-level gate metadata
 * so none of the previous account's profile, device, billing, or notice fields survive sign-out
 * or account deletion (Rule 23, AST-010).
 */
export function buildSignedOutAccountStatus(
  current: AccountStatusPayload | undefined,
  reason: "signed_out" | "account_deleted" = "signed_out"
): AccountStatusPayload | undefined {
  if (!current) return undefined;
  return {
    gate: current.gate,
    grace: current.grace,
    state: "signed_out",
    source: null,
    signed_out_reason: reason,
    pre_account_install: current.pre_account_install,
    installation_linked: false,
    token_name: null,
    signed_in: false,
    email: "",
    name: "",
    provider: "",
    device: null,
    marketing_opt_in: false,
    plan: "free",
    subscription: null,
    remote_computers_used: 0,
    remote_computers_limit: 1,
    cloud_routes_used: 0,
    cloud_routes_limit: 0,
    payment_issue: false,
    recent_sign_ins: [],
  };
}

/** Projects a live `/api/account` response into the `AccountStateFixture` view model. */
export function toAccountView(
  status: AccountStatusPayload,
  options: { cloudUnreachable?: boolean } = {}
): AccountStateFixture {
  const plan = normalizePlan(status.plan);
  const subscription = status.subscription ?? null;
  const subscriptionSource =
    subscription?.source === "complimentary"
      ? "complimentary"
      : subscription?.source === "stripe"
        ? "stripe"
        : null;
  const subscriptionInterval =
    subscription?.interval === "year" || subscription?.interval === "month"
      ? subscription.interval
      : null;
  const cancelAtPeriodEnd = Boolean(subscription?.cancel_at_period_end);
  const periodEnd = subscription?.current_period_end ?? null;
  const refundEligibleUntil = subscription?.refund_eligible_until ?? null;
  const isComplimentary = subscriptionSource === "complimentary";
  const fallbackName = status.name?.trim() || status.email?.split("@")[0] || "Account";

  return {
    signedIn: status.signed_in,
    accountId: status.email || status.device?.id || "account",
    name: fallbackName,
    email: status.email ?? "",
    provider: normalizeProvider(status.provider),
    source: normalizeSource(status.source),
    tokenName: status.token_name ?? null,
    marketingOptIn: status.marketing_opt_in,
    installationLinked: status.installation_linked,
    plan,
    remoteComputersUsed: status.remote_computers_used,
    remoteComputersLimit: status.remote_computers_limit,
    cloudRoutesUsed: status.cloud_routes_used,
    cloudRoutesLimit: status.cloud_routes_limit,
    paymentIssue: status.payment_issue,
    cancelAtPeriodEnd,
    renewsAt: !cancelAtPeriodEnd && !isComplimentary ? periodEnd : null,
    endsAt: cancelAtPeriodEnd || isComplimentary ? periodEnd : null,
    subscriptionSource,
    subscriptionInterval,
    refundEligibleUntil,
    formerSubscriber: false,
    cloudUnreachable: Boolean(options.cloudUnreachable),
    hostedRouteGraceDaysLeft: null,
  };
}

export const COMPOZY_TERMS_URL = "https://compozy.com/legal/terms";
export const COMPOZY_PRIVACY_URL = "https://compozy.com/legal/privacy";
export const COMPOZY_REFUNDS_URL = "https://compozy.com/legal/refunds";
export const COMPOZY_WHY_SIGN_IN_URL = "https://compozy.com/docs/account/sign-in";
export const COMPOZY_STATUS_URL = "https://status.compozy.com";
export const COMPOZY_SUPPORT_MAILTO = "mailto:support@compozy.com";

const REFUND_WINDOW_MS = 14 * 24 * 60 * 60 * 1000;

export interface AccountDeleteProInfo {
  source: "stripe" | "complimentary";
  interval: "year" | "month" | null;
  currentPeriodEnd: string | null;
  refundEligibleUntil: string | null;
}

export interface AccountDeleteProImpact {
  heading: string;
  periodLine: string;
  refundLine: string | null;
  refundEligible: boolean;
  confirmLabel: string;
  showRefundPolicyLink: boolean;
}

function formatUtcFullDate(iso: string | null): string | null {
  if (!iso) return null;
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

function formatUtcShortDate(ms: number): string {
  return new Date(ms).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
}

/**
 * Projects an active Pro subscription into the exact copy required by `_uiux.md` U3,
 * US-028 AC-4/EC-1, and Rule 27 (reused by S15's Delete account dialog).
 */
export function describeDeleteAccountProImpact(
  subscription: AccountDeleteProInfo,
  now: Date = new Date()
): AccountDeleteProImpact {
  const heading = "Deleting your account also cancels Pro.";
  const confirmLabel = "Delete account and cancel Pro";

  if (subscription.source === "complimentary") {
    return {
      heading,
      periodLine: "Your complimentary Pro ends now.",
      refundLine: null,
      refundEligible: false,
      confirmLabel,
      showRefundPolicyLink: false,
    };
  }

  const intervalSuffix =
    subscription.interval === "year"
      ? " (annual)"
      : subscription.interval === "month"
        ? " (monthly)"
        : "";
  const paidUntil = formatUtcFullDate(subscription.currentPeriodEnd);
  const periodLine = paidUntil
    ? `Pro${intervalSuffix} is paid until ${paidUntil} and ends now.`
    : `Pro${intervalSuffix} ends now.`;

  const refundUntilMs = subscription.refundEligibleUntil
    ? Date.parse(subscription.refundEligibleUntil)
    : Number.NaN;
  const refundEligible = !Number.isNaN(refundUntilMs) && refundUntilMs > now.getTime();
  const refundLine = refundEligible
    ? `Your last payment on ${formatUtcShortDate(refundUntilMs - REFUND_WINDOW_MS)} is refunded in full.`
    : "Refunds cover 14 days after a payment, so no refund applies.";

  return {
    heading,
    periodLine,
    refundLine,
    refundEligible,
    confirmLabel,
    showRefundPolicyLink: true,
  };
}

/** Extracts `AccountDeleteProInfo` when the account has active Pro entitlement. */
export function accountDeleteProInfoFromView(
  account: AccountStateFixture
): AccountDeleteProInfo | null {
  if (account.plan !== "pro") return null;
  return {
    source: account.subscriptionSource === "complimentary" ? "complimentary" : "stripe",
    interval: account.subscriptionInterval ?? null,
    currentPeriodEnd: account.endsAt ?? account.renewsAt,
    refundEligibleUntil: account.refundEligibleUntil ?? null,
  };
}

/** Maps billing API errors to plain product copy from `_dx.md`. */
export function describeBillingApiError(error: unknown, fallback: string): string {
  if (error instanceof AccountApiError) {
    switch (error.code) {
      case "account_cloud_unreachable":
      case "cloud_unavailable":
        return "Can't reach Compozy Cloud. Local work is unaffected.";
      case "account_suspended":
        return "This account is suspended. Contact support@compozy.com.";
      case "account_deleting":
        return "Your account is being deleted. Try again in a few minutes.";
      case "account_token_local_only":
        return "Account tokens unlock local use only. Sign in with Google or GitHub for this.";
      case "rate_limited":
        return "Too many requests. Try again in a moment.";
      case "already_pro":
        return "You're already on Pro.";
      default:
        return error.message || fallback;
    }
  }
  return error instanceof Error ? error.message : fallback;
}

/** Projects a live `/api/account/devices` item into `AccountDeviceFixture`. */
export function toAccountDeviceView(
  device: AccountDevicePayload,
  scopesOnLocal: readonly RemoteScope[] = []
): AccountDeviceFixture {
  return {
    id: device.id,
    name: device.name,
    kind: normalizeDeviceKind(device.kind),
    admin: device.admin,
    thisDevice: device.this_device,
    lastSeenAt: device.last_seen_at,
    lastSeenLabel: formatRelativeTimeLabel(device.last_seen_at),
    keyFingerprint: "",
    scopesOnLocal,
  };
}

/** Projects a live `/api/account/approvals` item into `PendingApprovalFixture`. */
export function toPendingApprovalView(
  approval: AccountApprovalPayload,
  computerNameById: ReadonlyMap<string, string>
): PendingApprovalFixture {
  const applied = approval.applied_result;
  const appliedResult =
    applied === "approved" || applied === "denied" || applied === "expired" ? applied : null;
  const state = appliedResult ?? normalizeApprovalState(approval.state);

  return {
    id: approval.id,
    computerId: approval.computer_id,
    computerName: computerNameById.get(approval.computer_id) ?? approval.computer_id,
    deviceName: approval.device_name,
    deviceKind: inferApprovalDeviceKind(approval.device_name),
    keyFingerprint: approval.key_fingerprint,
    createdAt: approval.created_at,
    createdLabel: formatRelativeTimeLabel(approval.created_at),
    state,
    appliedResult,
    canDecide: approval.can_decide,
  };
}

export interface AccountSignInNoticeView {
  deviceId: string;
  name: string;
  kind: DeviceKind;
  location: string;
  at: string;
  whenLabel: string;
}

/** Projects a live `/api/account` `recent_sign_ins` entry into the U3 notice view model. */
export function toAccountSignInNoticeView(
  notice: AccountSignInNoticePayload
): AccountSignInNoticeView {
  return {
    deviceId: notice.device_id,
    name: notice.name,
    kind: normalizeDeviceKind(notice.kind),
    location: notice.location,
    at: notice.at,
    whenLabel: formatRelativeTimeLabel(notice.at),
  };
}

/** Projects a live `/api/account/tokens` item into `AccountTokenFixture`. */
export function toAccountTokenView(token: AccountTokenPayload): AccountTokenFixture {
  return {
    id: token.id,
    name: token.name,
    createdLabel: "",
    lastUsedLabel: token.last_used_at ? formatRelativeTimeLabel(token.last_used_at) : null,
  };
}

/** Formats a grace-period deadline (`Dec 3, 14:00` or date) for the grace banner. */
export function formatGraceUntil(iso: string | null | undefined): string {
  if (!iso) return "later";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone: "UTC",
  });
}
