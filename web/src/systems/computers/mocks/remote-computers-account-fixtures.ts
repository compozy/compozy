import {
  ALL_REMOTE_SCOPES,
  type AccountDeviceFixture,
  type AccountPresetKey,
  type AccountStateFixture,
  type AccountUnaddedComputerFixture,
  type PendingApprovalFixture,
} from "./remote-computers-types";

export const OWNER_NAME = "Pedro Nauck";
export const OWNER_EMAIL = "pedro@example.com";
export const OWNER_ACCOUNT_ID = "acc_01k8m4p2q9";

const PRO_BASE: AccountStateFixture = {
  signedIn: true,
  accountId: OWNER_ACCOUNT_ID,
  name: OWNER_NAME,
  email: OWNER_EMAIL,
  provider: "google",
  source: "session",
  tokenName: null,
  marketingOptIn: true,
  installationLinked: true,
  plan: "pro",
  remoteComputersUsed: 9,
  remoteComputersLimit: null,
  cloudRoutesUsed: 3,
  cloudRoutesLimit: 10,
  paymentIssue: false,
  cancelAtPeriodEnd: false,
  renewsAt: "2026-12-22T00:00:00Z",
  endsAt: null,
  formerSubscriber: false,
  cloudUnreachable: false,
  hostedRouteGraceDaysLeft: null,
};

const FREE_BASE: AccountStateFixture = {
  ...PRO_BASE,
  marketingOptIn: false,
  plan: "free",
  remoteComputersUsed: 1,
  remoteComputersLimit: 1,
  cloudRoutesUsed: 0,
  cloudRoutesLimit: 0,
  renewsAt: null,
};

export const ACCOUNT_PRESETS: Record<AccountPresetKey, AccountStateFixture> = {
  pro: PRO_BASE,
  free: FREE_BASE,
  downgraded: { ...FREE_BASE, formerSubscriber: true, hostedRouteGraceDaysLeft: 5 },
  past_due: { ...PRO_BASE, paymentIssue: true, renewsAt: "2026-11-20T00:00:00Z" },
  ending: { ...PRO_BASE, cancelAtPeriodEnd: true, renewsAt: null, endsAt: "2026-12-22T00:00:00Z" },
  signed_out: {
    ...FREE_BASE,
    signedIn: false,
    accountId: "",
    name: "",
    email: "",
    provider: null,
    installationLinked: false,
    remoteComputersUsed: 0,
  },
  cloud_unreachable: { ...PRO_BASE, cloudUnreachable: true },
  token_installation: {
    ...FREE_BASE,
    provider: null,
    source: "account_token",
    tokenName: "ci",
    installationLinked: false,
    remoteComputersUsed: 0,
  },
  link_only_installation: {
    ...PRO_BASE,
    provider: null,
    source: "host_link",
    tokenName: null,
    installationLinked: true,
  },
};

export const ACCOUNT_DEVICES: readonly AccountDeviceFixture[] = [
  {
    id: "dev_2m8x0c",
    name: "MacBook Pro",
    kind: "desktop",
    admin: true,
    thisDevice: true,
    lastSeenAt: "2026-11-22T20:44:00Z",
    lastSeenLabel: "now",
    keyFingerprint: "SHA256:9aK2f8B1c4D7e0F3g6H9j2K5m8N1p4Q7v4Lm",
    scopesOnLocal: ALL_REMOTE_SCOPES,
  },
  {
    id: "dev_5k1p9z",
    name: "iPhone (Safari)",
    kind: "browser",
    admin: false,
    thisDevice: false,
    lastSeenAt: "2026-11-22T20:30:00Z",
    lastSeenLabel: "14m ago",
    keyFingerprint: "SHA256:4pR7m2N9q1W4e6T8y0U3i5O7p9A2s4D6x8Qn",
    scopesOnLocal: ["sessions:operate", "files:read"],
  },
  {
    id: "dev_8n3t2w",
    name: "Mac Studio",
    kind: "desktop",
    admin: true,
    thisDevice: false,
    lastSeenAt: "2026-11-22T18:44:00Z",
    lastSeenLabel: "2h ago",
    keyFingerprint: "SHA256:7cD1v5B9n3M7k1J5h9G3f7D1s5A9q3W7m2Wp",
    scopesOnLocal: ALL_REMOTE_SCOPES,
  },
  {
    id: "dev_6j4v1b",
    name: "iPad Air (Safari)",
    kind: "browser",
    admin: false,
    thisDevice: false,
    lastSeenAt: "2026-11-19T14:10:00Z",
    lastSeenLabel: "3d ago",
    keyFingerprint: "SHA256:2fH9k4L8z2X6c0V4b8N2m6Q0w4E8r2T6q5Yz",
    scopesOnLocal: ["sessions:operate", "terminal:operate", "files:read"],
  },
];

export const PENDING_APPROVALS: readonly PendingApprovalFixture[] = [
  {
    id: "apr_01k9a1",
    computerId: "local",
    computerName: "MacBook Pro",
    deviceName: "iPad Mini (Safari)",
    deviceKind: "browser",
    keyFingerprint: "SHA256:8tM3x6C9v2B5n8M1k4J7h0G3f6D9s2A5p1Kx",
    createdAt: "2026-11-22T20:42:00Z",
    createdLabel: "2m ago",
    state: "pending",
    appliedResult: null,
    canDecide: true,
  },
  {
    id: "apr_01k9a2",
    computerId: "cmp_1c9d0a2b",
    computerName: "studio",
    deviceName: "Pixel 9 (Chrome)",
    deviceKind: "browser",
    keyFingerprint: "SHA256:5wQ6r9T2y5U8i1O4p7A0s3D6f9G2h5J8r9Nv",
    createdAt: "2026-11-22T20:40:00Z",
    createdLabel: "4m ago",
    state: "pending",
    appliedResult: null,
    canDecide: false,
  },
];

export const ACCOUNT_UNADDED_COMPUTERS: readonly AccountUnaddedComputerFixture[] = [
  {
    id: "cmp_5t1y8h2j",
    name: "win-desktop",
    platform: "windows_x86_64",
    iconKind: "windows",
    online: true,
    hostedOnly: false,
    linkedVia: "command",
  },
  {
    id: "cmp_2d7n4k6v",
    name: "homelab",
    platform: "linux_arm64",
    iconKind: "server",
    online: false,
    hostedOnly: false,
    linkedVia: "device",
  },
];

export const ACCOUNT_UNADDED_FREE: readonly AccountUnaddedComputerFixture[] = [
  {
    id: "cmp_5t1y8h2j",
    name: "win-desktop",
    platform: "windows_x86_64",
    iconKind: "windows",
    online: true,
    hostedOnly: true,
    linkedVia: "command",
  },
];

/** Builds a computer fixture from `devbox` with deterministic overrides for tests and stories. */
