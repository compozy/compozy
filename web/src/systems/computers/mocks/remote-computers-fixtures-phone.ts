/**
 * Phone browser (Screen 07 / U8) fixture extensions.
 *
 * Imports and extends the canonical Remote Computers shared fixtures without
 * redefining any existing computer, device, peer, or plan.
 */
import {
  ACCOUNT_DEVICES,
  ACCOUNT_PRESETS,
  COMPUTERS_PRO,
  HOSTED_PEERS,
  OWNER_EMAIL,
  PENDING_APPROVALS,
  UNIFIED_SESSIONS,
  type ComputerFixture,
  type HostedPeerFixture,
  type RemoteScope,
  type UnifiedSessionFixture,
} from "./remote-computers-fixtures";

export const PHONE_TARGET_COMPUTER: ComputerFixture = COMPUTERS_PRO.find(
  item => item.id === "cmp_1c9d0a2b"
)!;

export const PHONE_HOSTED_URL = PHONE_TARGET_COMPUTER.hostedUrl ?? "studio-1c9d.compozy.run";

export const PHONE_DEVICE = ACCOUNT_DEVICES.find(item => item.id === "dev_5k1p9z")!;

export const PHONE_OWNER_EMAIL = OWNER_EMAIL;
export const PHONE_OTHER_ACCOUNT_EMAIL = "work@acme.com";

export const PHONE_GRACE_DAYS_LEFT = ACCOUNT_PRESETS.downgraded.hostedRouteGraceDaysLeft ?? 5;

export const PHONE_PEERS_MULTI: readonly HostedPeerFixture[] = HOSTED_PEERS;

export const PHONE_PEERS_SINGLE: readonly HostedPeerFixture[] = HOSTED_PEERS.filter(
  peer => peer.current
);

export const PHONE_FULL_SCOPES: readonly RemoteScope[] = [
  "sessions:operate",
  "terminal:operate",
  "files:read",
  "files:write",
  "tasks:operate",
];

/** Narrowed phone scopes matching `dev_5k1p9z` (no `terminal:operate`). */
export const PHONE_NARROW_SCOPES: readonly RemoteScope[] = PHONE_DEVICE.scopesOnLocal;

const BASE_STUDIO_SESSIONS = UNIFIED_SESSIONS.filter(
  item => item.computerId === PHONE_TARGET_COMPUTER.id
);

/**
 * Sessions running on `studio`, ordered with attention first, then active,
 * then recent settled sessions. Never includes sessions from other computers
 * (US-022 AC-4: a phone browser session reaches only its own origin).
 */
export const PHONE_STUDIO_SESSIONS: readonly UnifiedSessionFixture[] = [
  ...BASE_STUDIO_SESSIONS.filter(item => item.state === "attention"),
  {
    id: "sess-studio-3",
    title: "Profile Q4 quantization memory spike",
    computerId: PHONE_TARGET_COMPUTER.id,
    computerName: PHONE_TARGET_COMPUTER.name,
    isLocal: false,
    project: "api",
    agent: "general",
    model: "Opus 4.6",
    state: "running",
    column: "in_progress",
    updatedLabel: "1m",
  },
  {
    id: "sess-studio-4",
    title: "Verify CoreML export on M3 Ultra",
    computerId: PHONE_TARGET_COMPUTER.id,
    computerName: PHONE_TARGET_COMPUTER.name,
    isLocal: false,
    project: "api",
    agent: "general",
    model: "Sonnet 4.6",
    state: "running",
    column: "in_progress",
    updatedLabel: "12m",
  },
  ...BASE_STUDIO_SESSIONS.filter(item => item.state !== "attention"),
  {
    id: "sess-studio-5",
    title: "Clean up stale embedding cache files",
    computerId: PHONE_TARGET_COMPUTER.id,
    computerName: PHONE_TARGET_COMPUTER.name,
    isLocal: false,
    project: "compozy",
    agent: "general",
    model: "Sonnet 4.6",
    state: "idle",
    column: "done",
    updatedLabel: "3h",
  },
];

export const PHONE_APPROVALS_ON_STUDIO = PENDING_APPROVALS.filter(
  item => item.computerId === PHONE_TARGET_COMPUTER.id
);

export const PHONE_PENDING_COMMAND = {
  tool: "Terminal command",
  command: "pytest tests/embeddings/ --gpu -q",
  cwd: "/Users/pedro/code/api",
  reason: "Verifies cosine-similarity tolerance after switching to batch size 64.",
} as const;

export interface PhoneTranscriptEntry {
  id: string;
  kind: "user" | "agent" | "tool" | "marker";
  text?: string;
  command?: string;
  toolName?: string;
  preview?: string;
  toolStatus?: "success" | "running" | "failed";
  toolOutput?: string;
  timestamp: string;
}

export const PHONE_LIVE_TRANSCRIPT: readonly PhoneTranscriptEntry[] = [
  {
    id: "msg-1",
    kind: "user",
    text: "Run the MLX embedding benchmark at batch sizes 32 and 64, and write a quick comparison against our baseline.",
    timestamp: "17:56",
  },
  {
    id: "msg-2",
    kind: "tool",
    toolName: "Ran",
    preview: "python bench/mlx_embed.py --batches 32,64",
    toolStatus: "success",
    toolOutput:
      "batch=32  throughput=1,420 tok/s  peak_mem=4.1 GB\nbatch=64  throughput=2,180 tok/s  peak_mem=6.8 GB\nbaseline  throughput=1,190 tok/s  peak_mem=5.4 GB",
    timestamp: "17:58",
  },
  {
    id: "msg-3",
    kind: "agent",
    text: "Batch 64 reaches 2,180 tok/s on studio (+83% over baseline) with 6.8 GB peak memory out of 128 GB unified RAM. Next I can commit the updated fixture table and run the full regression suite.",
    timestamp: "17:59",
  },
  {
    id: "msg-4",
    kind: "marker",
    text: "Waiting for your approval to run",
    command: PHONE_PENDING_COMMAND.command,
    timestamp: "18:00",
  },
];

export const PHONE_TERMINAL_OUTPUT = [
  "pedro@studio ~/code/api (main) $ git status -sb",
  "## main...origin/main",
  " M bench/mlx_embed.py",
  " M bench/results/studio-m3-ultra.json",
  "pedro@studio ~/code/api (main) $ tail -n 3 bench/results/studio-m3-ultra.json",
  '  "batch_64_tok_s": 2180,',
  '  "peak_mem_gb": 6.8,',
  '  "baseline_delta": "+83%"',
  "pedro@studio ~/code/api (main) $",
] as const;
