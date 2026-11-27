import {
  ALL_REMOTE_SCOPES,
  type ComputerFixture,
  type ComputerRouteFixture,
  type ComputerRouteKind,
  type HostedPeerFixture,
  type SshHostSuggestionFixture,
  type UnifiedSessionFixture,
  type UpgradeMigrationNoticeFixture,
} from "./remote-computers-types";

const GB = 1024 * 1024 * 1024;
const OWNER_EMAIL = "pedro@example.com";

export const SSH_HOST_SUGGESTIONS: readonly SshHostSuggestionFixture[] = [
  {
    alias: "staging-box",
    hostname: "10.0.0.71",
    user: "deploy",
    port: 22,
    source: "ssh_config",
  },
  {
    alias: "ml-node",
    hostname: "10.0.0.94",
    user: "ubuntu",
    port: 22,
    source: "ssh_config",
  },
  {
    alias: "devbox",
    hostname: "10.0.0.42",
    user: "pedro",
    port: 22,
    source: "ssh_config",
    alreadyAdded: true,
  },
  {
    alias: "win-vm",
    hostname: "10.0.0.99",
    user: "admin",
    port: 22,
    source: "known_hosts",
    unsupportedPlatform: true,
  },
];

export const HOSTED_PEERS: readonly HostedPeerFixture[] = [
  {
    id: "cmp_1c9d0a2b",
    name: "studio",
    platform: "darwin_arm64",
    iconKind: "desktop",
    hostname: "studio-1c9d.compozy.run",
    online: true,
    current: true,
  },
  {
    id: "cmp_7f3k2q9x",
    name: "devbox",
    platform: "linux_x86_64",
    iconKind: "server",
    hostname: "devbox-7f3k.compozy.run",
    online: true,
  },
  {
    id: "local",
    name: "MacBook Pro",
    platform: "darwin_arm64",
    iconKind: "laptop",
    hostname: "macbook-2m8x.compozy.run",
    online: true,
  },
];

export const UNIFIED_SESSIONS: readonly UnifiedSessionFixture[] = [
  {
    id: "sess-local-1",
    title: "Polish settings navigation",
    computerId: "local",
    computerName: "This computer",
    isLocal: true,
    project: "compozy",
    agent: "general",
    model: "Sonnet 4.6",
    state: "running",
    column: "in_progress",
    updatedLabel: "2m",
  },
  {
    id: "sess-local-2",
    title: "Audit webhook ingress bindings",
    computerId: "local",
    computerName: "This computer",
    isLocal: true,
    project: "api",
    agent: "general",
    model: "Opus 4.6",
    state: "done",
    column: "in_review",
    prNumber: 418,
    updatedLabel: "18m",
  },
  {
    id: "sess-devbox-1",
    title: "Fix flaky Postgres pool tests",
    computerId: "cmp_7f3k2q9x",
    computerName: "devbox",
    isLocal: false,
    project: "api",
    agent: "general",
    model: "Sonnet 4.6",
    state: "running",
    column: "in_progress",
    updatedLabel: "1m",
  },
  {
    id: "sess-devbox-2",
    title: "Migrate Stripe webhook handler",
    computerId: "cmp_7f3k2q9x",
    computerName: "devbox",
    isLocal: false,
    project: "billing-worker",
    agent: "general",
    model: "Codex",
    state: "attention",
    column: "needs_you",
    updatedLabel: "4m",
  },
  {
    id: "sess-studio-1",
    title: "Benchmark local MLX embeddings",
    computerId: "cmp_1c9d0a2b",
    computerName: "studio",
    isLocal: false,
    project: "api",
    agent: "general",
    model: "Opus 4.6",
    state: "attention",
    column: "needs_you",
    updatedLabel: "6m",
  },
  {
    id: "sess-studio-2",
    title: "Generate release changelog",
    computerId: "cmp_1c9d0a2b",
    computerName: "studio",
    isLocal: false,
    project: "compozy",
    agent: "general",
    model: "Sonnet 4.6",
    state: "done",
    column: "done",
    updatedLabel: "1h",
  },
  {
    id: "sess-workstation-1",
    title: "Refactor GPU batch scheduler",
    computerId: "cmp_4b2m9x1p",
    computerName: "workstation",
    isLocal: false,
    computerOffline: true,
    project: "trainer",
    agent: "general",
    model: "Opus 4.6",
    state: "idle",
    column: "in_progress",
    updatedLabel: "38m",
  },
];

export const UPGRADE_NOTICE_FIXTURE: UpgradeMigrationNoticeFixture = {
  key: "gateway_removed",
  devicesRemoved: 3,
  providersRemoved: 1,
  connectionProfilesRemoved: 2,
  bindingsNeedingConfirmation: 2,
};

const SCALE_PREFIXES = [
  "build",
  "runner",
  "worker",
  "staging",
  "arm-ci",
  "eval",
  "mac-mini",
  "eu-west",
  "us-east",
  "sandbox",
] as const;

/** Generates additional deterministic computers after `seed` up to `count` total rows. */
export function buildScaleComputers(
  seed: readonly ComputerFixture[],
  count: number
): ComputerFixture[] {
  const result: ComputerFixture[] = [...seed];
  for (let index = result.length; index < count; index += 1) {
    const num = index - result.length + 1;
    const prefix = SCALE_PREFIXES[index % SCALE_PREFIXES.length]!;
    const name = `${prefix}-${String(num).padStart(2, "0")}`;
    const isMac = prefix === "mac-mini";
    const isOffline = index % 9 === 0;
    const isDisabled = !isOffline && index % 13 === 0;
    const isCloud = !isOffline && !isDisabled && index % 5 === 0;
    const ipLast = 100 + (index % 140);
    const routeKind: ComputerRouteKind = isCloud ? "cloud" : index % 2 === 0 ? "direct" : "ssh";
    const address =
      routeKind === "cloud"
        ? `${name}.compozy.run`
        : routeKind === "direct"
          ? `10.12.0.${ipLast}:47823`
          : `deploy@10.12.0.${ipLast}`;
    const label =
      routeKind === "cloud"
        ? "via hosted route"
        : routeKind === "direct"
          ? "via LAN"
          : `via SSH deploy@10.12.0.${ipLast}`;
    const route: ComputerRouteFixture = {
      id: `rt_scale_${num}`,
      kind: routeKind,
      address,
      label,
      inUse: !isOffline && !isDisabled,
      learned: routeKind === "direct",
      lastError: isOffline ? `Dial timed out on 10.12.0.${ipLast}` : null,
    };
    const cpuUtil = (((index * 17) % 72) + 8) / 100;
    const memTotal = (index % 3 === 0 ? 64 : 32) * GB;
    const memAvail = Math.round(memTotal * (1 - (((index * 13) % 60) + 15) / 100));

    result.push({
      id: `cmp_scale_${String(num).padStart(3, "0")}`,
      name,
      platform: isMac ? "darwin_arm64" : index % 3 === 0 ? "linux_arm64" : "linux_x86_64",
      iconKind: isMac ? "desktop" : prefix === "eval" ? "gpu" : "server",
      version: "0.8.0",
      protocol: { min: 1, max: 1 },
      status: isOffline ? "offline" : isDisabled ? "disabled" : "connected",
      enabled: !isDisabled,
      planState: "ok",
      originKind: routeKind === "ssh" ? "ssh" : "account",
      sshTarget: routeKind === "ssh" ? `deploy@10.12.0.${ipLast}` : undefined,
      routeInUse: !isOffline && !isDisabled ? route : null,
      routes: [route],
      scopes: ALL_REMOTE_SCOPES,
      load:
        isOffline || isDisabled
          ? null
          : {
              cpuCount: isMac ? 12 : 16,
              cpuUtil,
              memTotalBytes: memTotal,
              memAvailableBytes: memAvail,
              sampledAt: "2026-11-22T20:44:40Z",
              sampledAgoLabel: `${(index % 11) + 2}s ago`,
            },
      loadBalancingWeight: 50,
      diagnosis: isOffline
        ? {
            failingStep: "route",
            code: "route_timeout",
            message: `Couldn't reach ${address}`,
            at: "2026-11-22T20:20:00Z",
            retryInSeconds: 22,
            attempt: 2,
            steps: [
              { step: "cloud", label: "Compozy Cloud", status: "done", at: "22s ago" },
              {
                step: "route",
                label: "Route",
                status: "failed",
                message: `Dial timed out on 10.12.0.${ipLast}`,
                at: "22s ago",
              },
              { step: "computer", label: "Computer", status: "skipped" },
              { step: "sign_in", label: "Sign-in", status: "skipped" },
            ],
          }
        : null,
      providers: [
        { id: "claude", name: "Claude Code", status: "ready", accountLabel: OWNER_EMAIL },
        { id: "codex", name: "Codex", status: "ready", accountLabel: OWNER_EMAIL },
        { id: "cursor", name: "Cursor", status: "not_installed" },
      ],
      projects: ["api"],
      addedAt: "2026-11-05T12:00:00Z",
      lastConnectedAt: "2026-11-22T20:44:40Z",
      lastConnectedLabel: isOffline ? "24m ago" : "just now",
    });
  }
  return result;
}
