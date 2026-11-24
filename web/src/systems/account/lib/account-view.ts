import type {
  AccountDeviceFixture,
  AccountPlan,
  AccountProvider,
  AccountSessionSource,
  AccountStateFixture,
  ApprovalState,
  DeviceKind,
  PendingApprovalFixture,
  RemoteScope,
} from "@/systems/computers/mocks/remote-computers-types";

import {
  AccountApiError,
  type AccountApprovalPayload,
  type AccountDevicePayload,
  type AccountStatusPayload,
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
  if (source === "account_token") return "account_token";
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

/** Projects a live `/api/account` response into the `AccountStateFixture` view model. */
export function toAccountView(
  status: AccountStatusPayload,
  options: { cloudUnreachable?: boolean } = {}
): AccountStateFixture {
  const plan = normalizePlan(status.plan);
  const subscription = status.subscription ?? null;
  const cancelAtPeriodEnd = Boolean(subscription?.cancel_at_period_end);
  const periodEnd = subscription?.current_period_end ?? null;
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
    renewsAt: !cancelAtPeriodEnd ? periodEnd : null,
    endsAt: cancelAtPeriodEnd ? periodEnd : null,
    formerSubscriber: false,
    cloudUnreachable: Boolean(options.cloudUnreachable),
    hostedRouteGraceDaysLeft: null,
  };
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
