/**
 * Lane B fixtures extending the canonical shared Remote Computers fixtures
 * for Screen 03 (Add computer, U2) and Screen 04 (Computer details, U7).
 *
 * Never redefines existing computers, devices, or plans — imports and extends
 * them from `./remote-computers-fixtures`.
 */
import type { StepProgressItem } from "../components/step-progress-list";
import {
  ALL_REMOTE_SCOPES,
  COMPUTERS_PRO,
  OWNER_EMAIL,
  type ComputerFixture,
  type ComputerRouteFixture,
} from "./remote-computers-fixtures";

export type AddComputerMethod = "ssh" | "account" | "command";

export type AddComputerSshStage =
  | "picker"
  | "running"
  | "trust_host"
  | "host_key_changed"
  | "password"
  | "agent_secret"
  | "plan_limit"
  | "failed"
  | "cancel_confirm"
  | "unsupported"
  | "done";

export const SSH_HOST_TRUST_FIXTURE = {
  alias: "staging-box",
  target: "deploy@10.0.0.71:22",
  keyType: "ED25519",
  // 36-char body = 9 groups of 4 (3 even rows in 3-column grid, zero orphan chunk)
  fingerprint: "SHA256:3vQ8mN2pR7xK4wL9cF1hJ6tB5yD0sA8eG2qU",
  previousFingerprint: "SHA256:1aZ9xC4vB7nM2kL5jH8gF3dS6aP0oI4uY7tR",
  verifyCommand: "ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub",
} as const;

export const SSH_HOST_KEY_CHANGED_FIXTURE = {
  alias: "devbox",
  host: "10.0.0.42",
  user: "pedro",
  port: "2222",
  pinEntry: "[10.0.0.42]:2222",
  target: "pedro@10.0.0.42:2222",
  keyType: "ED25519",
  // 36-char bodies = 9 groups of 4 (3 even rows in 3-column grid, zero orphan chunk)
  pinnedFingerprint: "SHA256:1aZ9xC4vB7nM2kL5jH8gF3dS6aP0oI4uY7tR",
  observedFingerprint: "SHA256:3vQ8mN2pR7xK4wL9cF1hJ6tB5yD0sA8eG2qU",
  verifyCommand: "ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub",
  recoveryCommand: "ssh-keygen -R '[10.0.0.42]:2222' -f ~/.compozy/ssh/known_hosts",
} as const;

export const SSH_SECRET_PROMPT_FIXTURE = {
  target: "deploy@10.0.0.71",
  promptLabel: "SSH password or verification code",
  countdownSeconds: 165,
  agentSessionTitle: "Fix flaky Postgres pool tests",
} as const;

export const SSH_DOWNLOAD_FAILURE_FIXTURE = {
  errorMessage: "Couldn't download Compozy on staging-box",
  outputLine: "curl: (6) Could not resolve host: releases.compozy.com",
} as const;

export const SSH_LINGER_NOTE_FIXTURE = {
  summary: "Runs only while you're logged in on staging-box",
  detail: "Enable Linux user lingering so agents keep running after SSH closes.",
  command: "sudo loginctl enable-linger deploy",
} as const;

export const LINK_WITH_COMMAND_FIXTURE = {
  command: "compozy remote link",
  detectedComputer: {
    id: "cmp_9k2m7x4p",
    name: "ubuntu-office",
    platform: "linux_x86_64" as const,
    iconKind: "server" as const,
    online: true,
    hostedOnly: false,
    linkedVia: "command" as const,
  },
};

export interface BuildSshSetupStepsOptions {
  stage: AddComputerSshStage;
  hostLabel?: string;
  targetAddress?: string;
  useExistingInstall?: boolean;
  trustPromptNode?: StepProgressItem["prompt"];
  secretPromptNode?: StepProgressItem["prompt"];
  planLimitPromptNode?: StepProgressItem["prompt"];
  unsupportedPromptNode?: StepProgressItem["prompt"];
  retryAction?: StepProgressItem["action"];
}

/**
 * Builds the canonical 7-step SSH setup sequence (`admit, verify, connect,
 * install, service, link, forward`) in product language (`_uiux.md` U2).
 */
export function buildSshSetupSteps({
  stage,
  hostLabel = "staging-box",
  targetAddress = "deploy@10.0.0.71",
  useExistingInstall = false,
  trustPromptNode,
  secretPromptNode,
  planLimitPromptNode,
  unsupportedPromptNode,
  retryAction,
}: BuildSshSetupStepsOptions): StepProgressItem[] {
  const installTitle = useExistingInstall
    ? "Using installed Compozy 0.8.0"
    : "Installing Compozy 0.8.0";

  if (stage === "plan_limit") {
    return [
      {
        id: "admit",
        title: "Checking your plan",
        status: "failed",
        detail: "Free · 1 of 1 used",
        caption: "Free includes 1 remote computer. Upgrade to Pro for unlimited computers.",
        prompt: planLimitPromptNode,
      },
      { id: "verify", title: "Verifying release", status: "pending" },
      { id: "connect", title: "Connecting over SSH", status: "pending" },
      { id: "install", title: installTitle, status: "pending" },
      { id: "service", title: "Starting Compozy service", status: "pending" },
      { id: "link", title: "Linking to your account", status: "pending" },
      { id: "forward", title: "Opening the connection", status: "pending" },
    ];
  }

  if (stage === "trust_host" || stage === "host_key_changed") {
    const isChanged = stage === "host_key_changed";
    return [
      { id: "admit", title: "Checking your plan", status: "done", detail: "Pro · Unlimited" },
      { id: "verify", title: "Verifying release", status: "done", detail: "v0.8.0 · Sigstore" },
      {
        id: "connect",
        title: "Connecting over SSH",
        status: isChanged ? "failed" : "needs_input",
        detail: targetAddress,
        caption: isChanged ? undefined : "Verify this computer's host key before continuing.",
        prompt: trustPromptNode,
      },
      { id: "install", title: installTitle, status: "pending" },
      { id: "service", title: "Starting Compozy service", status: "pending" },
      { id: "link", title: "Linking to your account", status: "pending" },
      { id: "forward", title: "Opening the connection", status: "pending" },
    ];
  }

  if (stage === "password" || stage === "agent_secret") {
    return [
      { id: "admit", title: "Checking your plan", status: "done", detail: "Pro · Unlimited" },
      { id: "verify", title: "Verifying release", status: "done", detail: "v0.8.0 · Sigstore" },
      {
        id: "connect",
        title: "Connecting over SSH",
        status: "needs_input",
        detail: targetAddress,
        caption: `${hostLabel} asked for a password or one-time code.`,
        prompt: secretPromptNode,
      },
      { id: "install", title: installTitle, status: "pending" },
      { id: "service", title: "Starting Compozy service", status: "pending" },
      { id: "link", title: "Linking to your account", status: "pending" },
      { id: "forward", title: "Opening the connection", status: "pending" },
    ];
  }

  if (stage === "unsupported") {
    return [
      { id: "admit", title: "Checking your plan", status: "done", detail: "Pro · Unlimited" },
      { id: "verify", title: "Verifying release", status: "done", detail: "v0.8.0 · Sigstore" },
      {
        id: "connect",
        title: "Connecting over SSH",
        status: "failed",
        detail: "admin@10.0.0.99 · windows_x86_64",
        caption:
          "This computer's system isn't supported over SSH. Link it with compozy remote link instead.",
        prompt: unsupportedPromptNode,
      },
      { id: "install", title: installTitle, status: "pending" },
      { id: "service", title: "Starting Compozy service", status: "pending" },
      { id: "link", title: "Linking to your account", status: "pending" },
      { id: "forward", title: "Opening the connection", status: "pending" },
    ];
  }

  if (stage === "failed") {
    return [
      { id: "admit", title: "Checking your plan", status: "done", detail: "Pro · Unlimited" },
      { id: "verify", title: "Verifying release", status: "done", detail: "v0.8.0 · Sigstore" },
      {
        id: "connect",
        title: "Connecting over SSH",
        status: "done",
        detail: `${targetAddress} · linux_x86_64`,
      },
      {
        id: "install",
        title: installTitle,
        status: "failed",
        caption: SSH_DOWNLOAD_FAILURE_FIXTURE.errorMessage,
        outputLine: SSH_DOWNLOAD_FAILURE_FIXTURE.outputLine,
        action: retryAction,
      },
      { id: "service", title: "Starting Compozy service", status: "pending" },
      { id: "link", title: "Linking to your account", status: "pending" },
      { id: "forward", title: "Opening the connection", status: "pending" },
    ];
  }

  if (stage === "done") {
    return [
      { id: "admit", title: "Checking your plan", status: "done", detail: "Pro · Unlimited" },
      { id: "verify", title: "Verifying release", status: "done", detail: "v0.8.0 · Sigstore" },
      {
        id: "connect",
        title: "Connecting over SSH",
        status: "done",
        detail: `${targetAddress} · linux_x86_64`,
      },
      { id: "install", title: installTitle, status: "done", detail: "v0.8.0" },
      {
        id: "service",
        title: "Starting Compozy service",
        status: "done",
        detail: "systemd --user",
      },
      { id: "link", title: "Linking to your account", status: "done", detail: OWNER_EMAIL },
      {
        id: "forward",
        title: "Opening the connection",
        status: "done",
        detail: `via SSH ${targetAddress}`,
      },
    ];
  }

  // `running` and `cancel_confirm`
  return [
    { id: "admit", title: "Checking your plan", status: "done", detail: "Pro · Unlimited" },
    { id: "verify", title: "Verifying release", status: "done", detail: "v0.8.0 · Sigstore" },
    {
      id: "connect",
      title: "Connecting over SSH",
      status: "done",
      detail: `${targetAddress} · linux_x86_64`,
    },
    {
      id: "install",
      title: installTitle,
      status: "running",
      detail: "Downloading binary…",
    },
    { id: "service", title: "Starting Compozy service", status: "pending" },
    { id: "link", title: "Linking to your account", status: "pending" },
    { id: "forward", title: "Opening the connection", status: "pending" },
  ];
}

// ---------------------------------------------------------------------------
// Screen 04 (Computer details, U7) extended fixtures
// ---------------------------------------------------------------------------

/** Canonical `devbox` from `COMPUTERS_PRO`. */
export const DETAILS_DEVBOX: ComputerFixture = COMPUTERS_PRO[2]!;

/** Canonical `mini-build` (`needs_attention` with Linux linger off) from `COMPUTERS_PRO`. */
export const DETAILS_MINI_BUILD: ComputerFixture = COMPUTERS_PRO[0]!;

/** Canonical `workstation` (`offline` with Route diagnosis + LAN mismatch) from `COMPUTERS_PRO`. */
export const DETAILS_WORKSTATION: ComputerFixture = COMPUTERS_PRO[4]!;

/** Canonical `gpu-vm` (`incompatible` 0.7.9 over SSH) from `COMPUTERS_PRO`. */
export const DETAILS_GPU_VM: ComputerFixture = COMPUTERS_PRO[5]!;

/**
 * Derived variant of `devbox` paused at the `Sign-in` hop waiting for device
 * approval (`_uiux.md` U7 diagnosis hop `sign_in`).
 */
export const DETAILS_SIGNIN_APPROVAL: ComputerFixture = {
  ...DETAILS_DEVBOX,
  status: "needs_attention",
  routeInUse: null,
  load: null,
  attentionReason: undefined,
  diagnosis: {
    failingStep: "sign_in",
    code: "device_pending_approval",
    message: "Waiting for approval on devbox.",
    at: "2026-11-22T20:43:30Z",
    whatToTry: [
      "Open Compozy on devbox (or a device already trusted on devbox) and approve MacBook Pro under Settings → Account.",
      "Compare the key fingerprint on both screens before approving.",
      "This request expires automatically after 10 minutes if left unanswered.",
    ],
    steps: [
      {
        step: "cloud",
        label: "Compozy Cloud",
        status: "done",
        message: "Account verified",
        at: "40s ago",
      },
      {
        step: "route",
        label: "Route",
        status: "done",
        message: "Connected via SSH pedro@10.0.0.42",
        at: "38s ago",
      },
      {
        step: "computer",
        label: "Computer",
        status: "done",
        message: "Compozy 0.8.0",
        at: "37s ago",
      },
      {
        step: "sign_in",
        label: "Sign-in",
        status: "failed",
        message: "Waiting for approval on devbox.",
        at: "36s ago",
      },
    ],
  },
};

/**
 * Derived variant of `gpu-vm` linked via account/command (`originKind: "account"`)
 * so the Update tab displays the manual `compozy update` command instead of SSH 1-click update.
 */
export const DETAILS_INCOMPATIBLE_MANUAL: ComputerFixture = {
  ...DETAILS_GPU_VM,
  originKind: "account",
  sshTarget: undefined,
  routes: [
    {
      id: "rt_gpu_lan",
      kind: "direct",
      address: "192.168.1.88:47823",
      label: "via LAN",
      inUse: false,
      learned: true,
      lastError: null,
    },
    {
      id: "rt_gpu_cloud",
      kind: "cloud",
      address: "gpu-vm-8e7f.compozy.run",
      label: "via hosted route",
      inUse: false,
      learned: false,
      lastError: null,
    },
  ],
  diagnosis: {
    failingStep: "computer",
    code: "computer_incompatible",
    message: "gpu-vm runs Compozy 0.7.9. Update it to connect.",
    at: "2026-11-22T20:43:00Z",
    whatToTry: ["Run `compozy update` directly on gpu-vm, then click Check version again."],
    steps: [
      {
        step: "cloud",
        label: "Compozy Cloud",
        status: "done",
        message: "Account verified",
        at: "1m ago",
      },
      {
        step: "route",
        label: "Route",
        status: "done",
        message: "LAN reachable at 192.168.1.88:47823",
        at: "1m ago",
      },
      {
        step: "computer",
        label: "Computer",
        status: "failed",
        message: "Runs Compozy 0.7.9 (this app requires 0.8.0+)",
        at: "1m ago",
      },
      { step: "sign_in", label: "Sign-in", status: "skipped", message: "Not checked" },
    ],
  },
};

const LONG_VOLUME_ROUTES: readonly ComputerRouteFixture[] = [
  {
    id: "rt_long_1",
    kind: "direct",
    address: "10.240.18.142:47823",
    label: "via LAN",
    inUse: true,
    learned: true,
    lastError: null,
  },
  {
    id: "rt_long_2",
    kind: "direct",
    address: "fd7a:115c:a1e0::ab12:47823",
    label: "via LAN",
    inUse: false,
    learned: true,
    lastError: null,
  },
  {
    id: "rt_long_3",
    kind: "ssh",
    address: "engineering-ops@eu-central-1-prod-gpu-runner-04.internal.corp.example.com:2222",
    label: "via SSH engineering-ops@eu-central-1-prod-gpu-runner-04.internal.corp.example.com",
    inUse: false,
    learned: false,
    lastError: null,
  },
  {
    id: "rt_long_4",
    kind: "direct",
    address: "192.168.44.19:47823",
    label: "via LAN",
    inUse: false,
    learned: true,
    lastError: "Different computer at this address",
    identityMismatch: true,
  },
  {
    id: "rt_long_5",
    kind: "direct",
    address: "172.16.90.8:47823",
    label: "via LAN",
    inUse: false,
    learned: true,
    lastError: "Dial timed out after 5s",
  },
  {
    id: "rt_long_6",
    kind: "cloud",
    address: "eu-central-prod-runner-04-9x2k.compozy.run",
    label: "via hosted route",
    inUse: false,
    learned: false,
    lastError: null,
  },
];

/**
 * Stress-test fixture for long computer names, long hostnames, and 6 routes
 * (`brief-lanes.md` Screen 04: "long names/volume").
 */
export const DETAILS_LONG_VOLUME: ComputerFixture = {
  ...DETAILS_DEVBOX,
  id: "cmp_9x2k4m8p1q7v3n",
  name: "eu-central-prod-inference-workstation-04",
  sshTarget: "engineering-ops@eu-central-1-prod-gpu-runner-04.internal.corp.example.com",
  routeInUse: LONG_VOLUME_ROUTES[0]!,
  routes: LONG_VOLUME_ROUTES,
  scopes: ALL_REMOTE_SCOPES,
  hostedUrl: "eu-central-prod-runner-04-9x2k.compozy.run",
};

export type UpdateRunState = "idle" | "running" | "provenance_failed" | "done";

export function buildSshUpdateSteps(
  state: UpdateRunState,
  computerName: string
): StepProgressItem[] {
  if (state === "provenance_failed") {
    return [
      {
        id: "verify",
        title: "Verifying release 0.8.0",
        status: "failed",
        caption: "Couldn't verify this Compozy release. Nothing was installed.",
      },
      { id: "install", title: "Installing Compozy 0.8.0", status: "pending" },
      { id: "restart", title: `Restarting service on ${computerName}`, status: "pending" },
      { id: "reconnect", title: "Opening the connection", status: "pending" },
    ];
  }
  if (state === "done") {
    return [
      {
        id: "verify",
        title: "Verifying release 0.8.0",
        status: "done",
        detail: "Sigstore verified",
      },
      { id: "install", title: "Installing Compozy 0.8.0", status: "done", detail: "v0.8.0" },
      {
        id: "restart",
        title: `Restarting service on ${computerName}`,
        status: "done",
        detail: "systemd --user",
      },
      { id: "reconnect", title: "Opening the connection", status: "done", detail: "Connected" },
    ];
  }
  if (state === "running") {
    return [
      {
        id: "verify",
        title: "Verifying release 0.8.0",
        status: "done",
        detail: "Sigstore verified",
      },
      { id: "install", title: "Installing Compozy 0.8.0", status: "running", detail: "Uploading…" },
      { id: "restart", title: `Restarting service on ${computerName}`, status: "pending" },
      { id: "reconnect", title: "Opening the connection", status: "pending" },
    ];
  }
  return [
    { id: "verify", title: "Verifying release 0.8.0", status: "pending" },
    { id: "install", title: "Installing Compozy 0.8.0", status: "pending" },
    { id: "restart", title: `Restarting service on ${computerName}`, status: "pending" },
    { id: "reconnect", title: "Opening the connection", status: "pending" },
  ];
}

export function buildHealthyDiagnosisSteps(computer: ComputerFixture): StepProgressItem[] {
  return [
    {
      id: "cloud",
      title: "Compozy Cloud",
      status: "done",
      detail: "4s ago",
      caption: "Account and device grant verified",
    },
    {
      id: "route",
      title: "Route",
      status: "done",
      detail: "4s ago",
      caption: computer.routeInUse
        ? `Connected ${computer.routeInUse.label}`
        : "Active route healthy",
    },
    {
      id: "computer",
      title: "Computer",
      status: "done",
      detail: "4s ago",
      caption: `Compozy ${computer.version}`,
    },
    {
      id: "sign_in",
      title: "Sign-in",
      status: "done",
      detail: "4s ago",
      caption: `Signed in with ${computer.scopes.length} granted permissions`,
    },
  ];
}
