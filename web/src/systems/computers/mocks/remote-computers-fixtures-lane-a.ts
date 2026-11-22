/**
 * Lane A (Screen 01 Account) fixture extensions.
 * Imports and extends the shared Remote Computers fixtures without redefining
 * existing computers, devices, or base account presets.
 */
import {
  ACCOUNT_DEVICES,
  ACCOUNT_PRESETS,
  PENDING_APPROVALS,
  type AccountDeviceFixture,
  type AccountStateFixture,
  type AccountTokenFixture,
  type PendingApprovalFixture,
} from "./remote-computers-fixtures";

export interface AccountDeviceFlowFixture {
  verificationUri: string;
  userCode: string;
  expiresLabel: string;
}

export interface ProPlanOptionFixture {
  interval: "year" | "month";
  title: string;
  price: string;
  cadenceDetail: string;
  badge?: string;
}

export const ACCOUNT_DEVICE_FLOW: AccountDeviceFlowFixture = {
  verificationUri: "https://cloud.compozy.com/activate",
  userCode: "WDJB-MJHT",
  expiresLabel: "Expires in 9m 42s",
};

/**
 * Pricing comes from `_uiux.md` U9 and `_user_stories.md` US-002 AC-1.
 */
export const ACCOUNT_PRO_PLAN_OPTIONS: readonly ProPlanOptionFixture[] = [
  {
    interval: "year",
    title: "Pro annual",
    price: "USD 79.99/year",
    cadenceDetail: "≈ USD 6.67/month, billed once a year",
    badge: "Best value · Save 33%",
  },
  {
    interval: "month",
    title: "Pro monthly",
    price: "USD 9.99/month",
    cadenceDetail: "Billed monthly. Cancel anytime.",
  },
];

export const ACCOUNT_PRO_BENEFITS: ReadonlyArray<{
  id: string;
  label: string;
  detail: string;
}> = [
  {
    id: "computers",
    label: "Unlimited computers",
    detail: "Connect as many Linux, macOS, and Windows machines as you own.",
  },
  {
    id: "hosted-route",
    label: "Hosted route: reach your computers from anywhere",
    detail: "Up to 10 computers reachable across any network without VPN setup.",
  },
  {
    id: "phone",
    label: "Phone access",
    detail: "Open and steer live agent sessions from Safari or Chrome on your phone.",
  },
  {
    id: "webhooks",
    label: "Hosted webhooks",
    detail: "Receive automation webhook triggers directly on your computers.",
  },
];

/**
 * Devices view when the current machine (`MacBook Pro`) is a standard (non-admin) device
 * while `Mac Studio` remains trusted for admin (`_dx.md` `account_admin_required`).
 */
export const ACCOUNT_NON_ADMIN_DEVICES: readonly AccountDeviceFixture[] = ACCOUNT_DEVICES.map(
  device => (device.thisDevice ? { ...device, admin: false } : device)
);

/**
 * Approvals queue showing every lifecycle state from `_dx.md` `GET /api/account/approvals`:
 * pending (actionable), delivering in flight, pending (non-decidable on this device),
 * and recently decided (within 24 hours).
 */
export const ACCOUNT_APPROVALS_EXTENDED: readonly PendingApprovalFixture[] = [
  PENDING_APPROVALS[0]!,
  {
    id: "apr_01k9a3",
    computerId: "cmp_7f3k2q9x",
    computerName: "devbox",
    deviceName: "iPhone (Firefox)",
    deviceKind: "browser",
    keyFingerprint: "SHA256:3kL9n2P5q8R1s4T7v0W3x6Y9z2A5b8C1d4Ef",
    createdAt: "2026-11-22T20:41:00Z",
    createdLabel: "3m ago",
    state: "delivering",
    appliedResult: null,
    canDecide: true,
  },
  PENDING_APPROVALS[1]!,
  {
    id: "apr_01k9a4",
    computerId: "local",
    computerName: "MacBook Pro",
    deviceName: "iPad Air (Safari)",
    deviceKind: "browser",
    keyFingerprint: "SHA256:2fH9k4L8z2X6c0V4b8N2m6Q0w4E8r2T6q5Yz",
    createdAt: "2026-11-22T18:15:00Z",
    createdLabel: "2h ago",
    state: "approved",
    appliedResult: "approved",
    canDecide: true,
  },
  {
    id: "apr_01k9a5",
    computerId: "cmp_7f3k2q9x",
    computerName: "devbox",
    deviceName: "Linux laptop (CLI)",
    deviceKind: "cli",
    keyFingerprint: "SHA256:6mN8p0Q2r4S6t8U0v2W4x6Y8z0A2b4C6e8Fg",
    createdAt: "2026-11-22T15:00:00Z",
    createdLabel: "5h ago",
    state: "expired",
    appliedResult: "expired",
    canDecide: true,
  },
];

/** Pro account at the 10-computer hosted-route fair-use cap (`cloud_routes_limit_reached`). */
export const ACCOUNT_PRO_HOSTED_CAP: AccountStateFixture = {
  ...ACCOUNT_PRESETS.pro,
  remoteComputersUsed: 12,
  cloudRoutesUsed: 10,
  cloudRoutesLimit: 10,
};

/** Account tokens for CI and ephemeral environments (`_uiux.md` U3, US-029). */
export const ACCOUNT_TOKENS: readonly AccountTokenFixture[] = [
  {
    id: "atk_8d2k1m",
    name: "ci",
    createdLabel: "Nov 12, 2026",
    lastUsedLabel: "2 hours ago",
  },
  {
    id: "atk_3p7r9x",
    name: "release-runner",
    createdLabel: "Nov 19, 2026",
    lastUsedLabel: "1 day ago",
  },
  {
    id: "atk_5n1q4v",
    name: "nightly-evals",
    createdLabel: "Nov 21, 2026",
    lastUsedLabel: null,
  },
];

export interface CreatedAccountTokenFixture {
  id: string;
  name: string;
  token: string;
}

export const CREATED_ACCOUNT_TOKEN_FIXTURE: CreatedAccountTokenFixture = {
  id: "atk_9w4m2k",
  name: "ci-preview",
  token: "cpzat_2Qe8kL4mN9pR7xT1vB6cD3fG5hJ0sA8wY2uXv9",
};
