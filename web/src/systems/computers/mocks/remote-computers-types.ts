/**
 * Pure domain types and constant lookup tables for Remote Computers views and fixtures.
 */

export type AccountPlan = "free" | "pro";

export type AccountProvider = "google" | "github";

export type AccountSessionSource = "session" | "host_link" | "account_token";

export type AccountPresetKey =
  | "pro"
  | "free"
  | "downgraded"
  | "past_due"
  | "ending"
  | "signed_out"
  | "cloud_unreachable"
  | "token_installation"
  | "link_only_installation";

export interface AccountTokenFixture {
  id: string;
  name: string;
  createdLabel: string;
  lastUsedLabel: string | null;
}

export interface AccountStateFixture {
  signedIn: boolean;
  accountId: string;
  name: string;
  email: string;
  provider: AccountProvider | null;
  source: AccountSessionSource;
  /** Token name when `source === "account_token"` (e.g. `"ci"`). */
  tokenName: string | null;
  marketingOptIn: boolean;
  /** True when this computer is also linked to the account for remote access. */
  installationLinked: boolean;
  plan: AccountPlan;
  remoteComputersUsed: number;
  /** `null` = unlimited (Pro). */
  remoteComputersLimit: number | null;
  cloudRoutesUsed: number;
  /** `0` = not included on plan (Free). */
  cloudRoutesLimit: number;
  paymentIssue: boolean;
  cancelAtPeriodEnd: boolean;
  renewsAt: string | null;
  endsAt: string | null;
  /** Subscription billing source from `/api/account` (`stripe` or `complimentary`). */
  subscriptionSource?: "stripe" | "complimentary" | null;
  /** Billing interval (`year` or `month`) when subscribed through Stripe. */
  subscriptionInterval?: "year" | "month" | null;
  /** ISO deadline for an automatic full refund on deletion (14 days from eligible payment). */
  refundEligibleUntil?: string | null;
  formerSubscriber: boolean;
  cloudUnreachable: boolean;
  /** Days remaining in the 7-day entitlement-only hosted-route window after downgrade. */
  hostedRouteGraceDaysLeft: number | null;
}

export type DeviceKind = "desktop" | "browser" | "cli";

export type RemoteScope =
  | "sessions:operate"
  | "terminal:operate"
  | "files:read"
  | "files:write"
  | "tasks:operate"
  | "settings:write"
  | "remote:admin";

export interface RemoteScopeMeta {
  scope: RemoteScope;
  label: string;
  consequence: string;
}

export const DISPLAYED_REMOTE_SCOPES: readonly RemoteScopeMeta[] = [
  {
    scope: "sessions:operate",
    label: "Sessions",
    consequence: "Read, start, steer, and stop agent sessions",
  },
  {
    scope: "terminal:operate",
    label: "Terminals",
    consequence: "Open and type in interactive terminals",
  },
  {
    scope: "files:read",
    label: "Read files",
    consequence: "Browse project files, diffs, and reviews",
  },
  {
    scope: "files:write",
    label: "Write files",
    consequence: "Edit files and manage git worktrees",
  },
  {
    scope: "tasks:operate",
    label: "Tasks",
    consequence: "Create, claim, and review tasks",
  },
  {
    scope: "settings:write",
    label: "Settings",
    consequence: "Change daemon settings and providers",
  },
] as const;

export const ALL_REMOTE_SCOPES: readonly RemoteScope[] = [
  "sessions:operate",
  "terminal:operate",
  "files:read",
  "files:write",
  "tasks:operate",
  "settings:write",
  "remote:admin",
];

export interface AccountDeviceFixture {
  id: string;
  name: string;
  kind: DeviceKind;
  admin: boolean;
  thisDevice: boolean;
  lastSeenAt: string;
  lastSeenLabel: string;
  keyFingerprint: string;
  scopesOnLocal: readonly RemoteScope[];
}

export type ApprovalState = "pending" | "delivering" | "approved" | "denied" | "expired";

export interface PendingApprovalFixture {
  id: string;
  computerId: string;
  computerName: string;
  deviceName: string;
  deviceKind: DeviceKind;
  keyFingerprint: string;
  createdAt: string;
  createdLabel: string;
  state: ApprovalState;
  appliedResult: "approved" | "denied" | "expired" | null;
  canDecide: boolean;
}

export type ComputerStatus =
  | "connected"
  | "connecting"
  | "offline"
  | "needs_attention"
  | "needs_pro"
  | "incompatible"
  | "disabled";

export type ComputerPlatform =
  | "darwin_arm64"
  | "darwin_x86_64"
  | "linux_x86_64"
  | "linux_arm64"
  | "windows_x86_64";

export type ComputerIconKind = "laptop" | "desktop" | "server" | "gpu" | "windows";

export type ComputerRouteKind = "direct" | "ssh" | "cloud";

export interface ComputerRouteFixture {
  id: string;
  kind: ComputerRouteKind;
  address: string;
  label: string;
  inUse: boolean;
  learned: boolean;
  lastError: string | null;
  identityMismatch?: boolean;
}

export interface ComputerLoadFixture {
  cpuCount: number;
  /** 0..1 */
  cpuUtil: number;
  memTotalBytes: number;
  memAvailableBytes: number;
  sampledAt: string;
  sampledAgoLabel: string;
  stale?: boolean;
}

export type DiagnosisHop = "cloud" | "route" | "computer" | "sign_in";

export interface ComputerDiagnosisStepFixture {
  step: DiagnosisHop;
  label: string;
  status: "done" | "failed" | "skipped" | "pending";
  message?: string;
  at?: string;
}

export interface ComputerDiagnosisFixture {
  failingStep: DiagnosisHop;
  code: string;
  message: string;
  at: string;
  retryInSeconds?: number;
  attempt?: number;
  whatToTry?: readonly string[];
  steps: readonly ComputerDiagnosisStepFixture[];
}

export interface ComputerProviderAuthFixture {
  id: "claude" | "codex" | "cursor";
  name: string;
  status: "ready" | "signed_out" | "not_installed";
  accountLabel?: string;
}

export type LoadBalancingWeight = 100 | 50 | 25 | 0;

export const LOAD_BALANCING_WEIGHT_OPTIONS: ReadonlyArray<{
  value: LoadBalancingWeight;
  label: string;
  shortLabel: string;
}> = [
  { value: 100, label: "Prefer", shortLabel: "prefer" },
  { value: 50, label: "Normal", shortLabel: "normal" },
  { value: 25, label: "Less often", shortLabel: "less often" },
  { value: 0, label: "Manual only", shortLabel: "manual only" },
];

export interface ComputerAttentionReasonFixture {
  kind: "linger_off" | "provider_signed_out";
  summary: string;
  detail: string;
  command?: string;
  actionLabel: string;
}

export interface ComputerFixture {
  id: string;
  name: string;
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  version: string;
  protocol: { min: number; max: number };
  status: ComputerStatus;
  enabled: boolean;
  planState: "ok" | "limited";
  originKind: "ssh" | "account";
  sshTarget?: string;
  routeInUse: ComputerRouteFixture | null;
  routes: readonly ComputerRouteFixture[];
  scopes: readonly RemoteScope[];
  load: ComputerLoadFixture | null;
  loadBalancingWeight: LoadBalancingWeight;
  attentionReason?: ComputerAttentionReasonFixture;
  diagnosis: ComputerDiagnosisFixture | null;
  providers: readonly ComputerProviderAuthFixture[];
  projects: readonly string[];
  addedAt: string;
  lastConnectedAt: string;
  lastConnectedLabel: string;
  hostedUrl?: string;
}

export type HostedRouteState = "ready" | "starting" | "unreachable" | "off";

export interface LocalComputerFixture {
  id: "local";
  name: string;
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  version: string;
  localRunAgents: boolean;
  activeLocalSessions: number;
  remoteEnabled: boolean;
  hostedRouteEnabled: boolean;
  hostedRouteState: HostedRouteState;
  hostedUrl: string;
  hostedRouteError?: string;
  requireDeviceApproval: boolean;
  load: ComputerLoadFixture;
  loadBalancingWeight: LoadBalancingWeight;
  projects: readonly string[];
}

export interface AccountUnaddedComputerFixture {
  id: string;
  name: string;
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  online: boolean;
  hostedOnly: boolean;
  linkedVia: "command" | "device";
}

export interface SshHostSuggestionFixture {
  alias: string;
  hostname: string;
  user: string;
  port: number;
  source: "ssh_config" | "known_hosts";
  alreadyAdded?: boolean;
  unsupportedPlatform?: boolean;
}

export interface HostedPeerFixture {
  id: string;
  name: string;
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  hostname: string;
  online: boolean;
  current?: boolean;
}

export interface UnifiedSessionFixture {
  id: string;
  title: string;
  computerId: string;
  computerName: string;
  isLocal: boolean;
  computerOffline?: boolean;
  project: string;
  agent: string;
  model: string;
  state: "running" | "attention" | "done" | "idle";
  column: "in_progress" | "needs_you" | "in_review" | "done";
  prNumber?: number;
  updatedLabel: string;
}

export interface UpgradeMigrationNoticeFixture {
  key: "gateway_removed";
  devicesRemoved: number;
  providersRemoved: number;
  connectionProfilesRemoved: number;
  bindingsNeedingConfirmation: number;
}
