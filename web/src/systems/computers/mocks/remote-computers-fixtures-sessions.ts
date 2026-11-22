/**
 * Lane C (Screens 05 Run on + 06 Sessions) fixture extensions.
 *
 * Imports and extends the shared Remote Computers fixtures without redefining
 * any computer, device, or account preset.
 */
import type {
  SessionBoardColumnId,
  SessionBoardPayload,
} from "@/systems/session/lib/session-board-contract";
import { primarySessionFixture, sessionRuntimeWithModel } from "@/systems/session/mocks/fixtures";
import type { SessionPayload } from "@/systems/session/types";

import {
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
  UNIFIED_SESSIONS,
  makeScaleComputers,
  type ComputerFixture,
  type ComputerIconKind,
  type ComputerLoadFixture,
  type ComputerPlatform,
  type UnifiedSessionFixture,
} from "./remote-computers-fixtures";

export interface ComputerProjectOption {
  id: string;
  name: string;
  path: string;
  repo: string;
  branch: string;
}

/** Projects per computer (same repo `github.com/pedro/api` on local, studio, and devbox). */
export const COMPUTER_PROJECTS_BY_ID: Readonly<Record<string, readonly ComputerProjectOption[]>> = {
  local: [
    {
      id: "ws_local_compozy",
      name: "compozy",
      path: "/Users/pedro/Dev/compozy/compozy",
      repo: "github.com/compozy/compozy",
      branch: "main",
    },
    {
      id: "ws_local_api",
      name: "api",
      path: "/Users/pedro/Dev/pedro/api",
      repo: "github.com/pedro/api",
      branch: "feat/webhook-ingress",
    },
  ],
  cmp_1c9d0a2b: [
    {
      id: "ws_studio_api",
      name: "api",
      path: "/Users/pedro/Code/api",
      repo: "github.com/pedro/api",
      branch: "perf/mlx-embeddings",
    },
    {
      id: "ws_studio_compozy",
      name: "compozy",
      path: "/Users/pedro/Code/compozy",
      repo: "github.com/compozy/compozy",
      branch: "main",
    },
  ],
  cmp_7f3k2q9x: [
    {
      id: "ws_devbox_api",
      name: "api",
      path: "/home/pedro/code/api",
      repo: "github.com/pedro/api",
      branch: "fix/pg-pool-flake",
    },
    {
      id: "ws_devbox_billing",
      name: "billing-worker",
      path: "/home/pedro/code/billing-worker",
      repo: "github.com/pedro/billing-worker",
      branch: "feat/stripe-v2",
    },
  ],
  cmp_9r5k3n7q: [
    {
      id: "ws_mini_compozy",
      name: "compozy",
      path: "/home/pedro/src/compozy",
      repo: "github.com/compozy/compozy",
      branch: "main",
    },
    {
      id: "ws_mini_api",
      name: "api",
      path: "/home/pedro/src/api",
      repo: "github.com/pedro/api",
      branch: "main",
    },
  ],
  cmp_3v8q1z4m: [
    {
      id: "ws_hetzner_infra",
      name: "infra",
      path: "/srv/deploy/infra",
      repo: "github.com/pedro/infra",
      branch: "main",
    },
  ],
  cmp_4b2m9x1p: [
    {
      id: "ws_ws_trainer",
      name: "trainer",
      path: "/home/pedro/ml/trainer",
      repo: "github.com/pedro/trainer",
      branch: "refactor/batch-scheduler",
    },
  ],
  cmp_8e7f6d5c: [
    {
      id: "ws_gpu_trainer",
      name: "trainer",
      path: "/home/ubuntu/trainer",
      repo: "github.com/pedro/trainer",
      branch: "main",
    },
    {
      id: "ws_gpu_evals",
      name: "evals",
      path: "/home/ubuntu/evals",
      repo: "github.com/pedro/evals",
      branch: "main",
    },
  ],
};

/** Projects available across all computers for Auto resolution. */
export const ALL_AUTO_PROJECTS: readonly ComputerProjectOption[] = [
  COMPUTER_PROJECTS_BY_ID.local![1]!, // api (on local, studio, devbox, mini-build)
  COMPUTER_PROJECTS_BY_ID.local![0]!, // compozy (on local, studio, mini-build)
  COMPUTER_PROJECTS_BY_ID.cmp_7f3k2q9x![1]!, // billing-worker (on devbox only)
  COMPUTER_PROJECTS_BY_ID.cmp_4b2m9x1p![0]!, // trainer (on workstation [offline] & gpu-vm [incompatible])
];

export function projectsForTarget(target: string): readonly ComputerProjectOption[] {
  if (target === "auto") return ALL_AUTO_PROJECTS;
  return COMPUTER_PROJECTS_BY_ID[target] ?? [];
}

/**
 * Computes which computer `Auto` resolves to for a given project name, matching
 * `_user_stories.md` US-015 (prefers eligible connected computer with highest
 * weighted free capacity; skips Manual only, offline, incompatible, and Needs Pro).
 */
export function resolveAutoComputer(
  projectName: string,
  localComputer: typeof LOCAL_COMPUTER,
  computers: readonly ComputerFixture[]
): {
  computerId: string;
  computerName: string;
  reason: string;
  candidates: Array<{
    id: string;
    name: string;
    load: ComputerLoadFixture | null;
    weightLabel: string;
    winner: boolean;
  }>;
} | null {
  type Candidate = {
    id: string;
    name: string;
    score: number;
    load: ComputerLoadFixture | null;
    weight: number;
  };
  const list: Candidate[] = [];

  if (
    localComputer.localRunAgents &&
    localComputer.loadBalancingWeight > 0 &&
    localComputer.projects.includes(projectName)
  ) {
    const cpuFree = 1 - localComputer.load.cpuUtil;
    const memFree = localComputer.load.memAvailableBytes / localComputer.load.memTotalBytes;
    list.push({
      id: "local",
      name: "This computer",
      score: ((cpuFree + memFree) / 2) * localComputer.loadBalancingWeight,
      load: localComputer.load,
      weight: localComputer.loadBalancingWeight,
    });
  }

  for (const computer of computers) {
    if (
      !computer.enabled ||
      (computer.status !== "connected" && computer.status !== "needs_attention") ||
      computer.loadBalancingWeight <= 0 ||
      !computer.projects.includes(projectName)
    ) {
      continue;
    }
    const cpuUtil = computer.load?.cpuUtil ?? 0.5;
    const memFreeRatio = computer.load
      ? computer.load.memAvailableBytes / computer.load.memTotalBytes
      : 0.5;
    list.push({
      id: computer.id,
      name: computer.name,
      score: ((1 - cpuUtil + memFreeRatio) / 2) * computer.loadBalancingWeight,
      load: computer.load,
      weight: computer.loadBalancingWeight,
    });
  }

  if (list.length === 0) return null;
  list.sort((a, b) => b.score - a.score);
  const best = list[0]!;
  const weightText = (w: number) =>
    w >= 100 ? "Prefer" : w >= 50 ? "Normal" : w >= 25 ? "Less often" : "Manual only";

  return {
    computerId: best.id,
    computerName: best.name,
    reason: "Picks the least busy computer",
    candidates: list.map((item, idx) => ({
      id: item.id,
      name: item.name,
      load: item.load,
      weightLabel: weightText(item.weight),
      winner: idx === 0,
    })),
  };
}

export interface RemoteSessionRowEntry extends UnifiedSessionFixture {
  iconKind: ComputerIconKind;
  platform: ComputerPlatform;
  provider: "claude" | "codex";
  branch?: string;
  needsYouPhrase?: string;
  minutesAgo: number;
}

const COMPUTER_LOOKUP = new Map<string, { iconKind: ComputerIconKind; platform: ComputerPlatform }>(
  [
    ["local", { iconKind: LOCAL_COMPUTER.iconKind, platform: LOCAL_COMPUTER.platform }],
    ...COMPUTERS_PRO.map(c => [c.id, { iconKind: c.iconKind, platform: c.platform }] as const),
  ]
);

const EXTRA_SESSION_DETAILS: Record<
  string,
  Pick<RemoteSessionRowEntry, "provider" | "branch" | "needsYouPhrase" | "minutesAgo">
> = {
  "sess-local-1": { provider: "claude", branch: "feat/settings-nav", minutesAgo: 2 },
  "sess-local-2": { provider: "claude", branch: "feat/webhook-ingress", minutesAgo: 18 },
  "sess-devbox-1": { provider: "claude", branch: "fix/pg-pool-flake", minutesAgo: 1 },
  "sess-devbox-2": {
    provider: "codex",
    branch: "feat/stripe-v2",
    needsYouPhrase: "Needs approval",
    minutesAgo: 4,
  },
  "sess-studio-1": {
    provider: "claude",
    branch: "perf/mlx-embeddings",
    needsYouPhrase: "Waiting for input",
    minutesAgo: 6,
  },
  "sess-studio-2": { provider: "claude", branch: "release/v0.8.0", minutesAgo: 60 },
  "sess-workstation-1": {
    provider: "claude",
    branch: "refactor/batch-scheduler",
    minutesAgo: 38,
  },
};

export const REMOTE_SESSION_ENTRIES: readonly RemoteSessionRowEntry[] = UNIFIED_SESSIONS.map(
  session => {
    const hardware = COMPUTER_LOOKUP.get(session.computerId) ?? {
      iconKind: "server" as const,
      platform: "linux_x86_64" as const,
    };
    const extra = EXTRA_SESSION_DETAILS[session.id] ?? {
      provider: "claude" as const,
      minutesAgo: 10,
    };
    return {
      ...session,
      ...hardware,
      ...extra,
    };
  }
);

const BASE_NOW = Date.parse("2026-11-22T20:45:00Z");

function toBoardColumn(column: UnifiedSessionFixture["column"]): SessionBoardColumnId {
  if (column === "in_progress") return "working";
  return column;
}

function toBadge(entry: RemoteSessionRowEntry): SessionPayload["badge"] {
  if (entry.computerOffline) return "idle";
  if (entry.state === "running") return "running";
  if (entry.state === "attention") return "waiting-for-input";
  if (entry.state === "done") return "done";
  return "idle";
}

/** Projects a `RemoteSessionRowEntry` into a real `SessionBoardPayload` for `SessionBoardCardView`. */
export function toBoardSessionPayload(entry: RemoteSessionRowEntry): SessionBoardPayload {
  const updatedIso = new Date(BASE_NOW - entry.minutesAgo * 60_000).toISOString();
  return {
    ...primarySessionFixture,
    id: entry.id,
    name: entry.title,
    agent_name: entry.agent,
    workspace_id: `ws_${entry.computerId}_${entry.project}`,
    workspace_path: `/code/${entry.project}`,
    runtime: sessionRuntimeWithModel(entry.provider),
    state: entry.computerOffline ? "stopped" : "active",
    badge: toBadge(entry),
    board_column: toBoardColumn(entry.column),
    branch: entry.branch,
    updated_at: updatedIso,
    archived_at: null,
    pull_request: entry.prNumber
      ? {
          provider: "github",
          number: entry.prNumber,
          url: `https://github.com/pedro/${entry.project}/pull/${entry.prNumber}`,
          state: "open",
          draft: false,
          head_sha: "4c9a12b8ef",
          checks: { state: "passing", passed: 14, failed: 0, pending: 0, failed_names: [] },
          review_decision: "review_required",
          mergeable: "mergeable",
          ready_to_merge: false,
        }
      : null,
    pull_request_lookup: entry.branch
      ? { state: "ok", cause: "", observed_at: updatedIso, stale: false }
      : null,
  };
}

/** Deterministic high-volume session list across 8 computers for Screen 06 scale state. */
export const VOLUME_SESSION_ENTRIES: readonly RemoteSessionRowEntry[] = (() => {
  const scaleComputers = makeScaleComputers(12).slice(0, 8);
  const titles = [
    "Verify migration rollback on staging DB",
    "Add retry jitter to control link reconnect",
    "Investigate worker memory spike under load",
    "Audit OAuth token refresh expiry edge cases",
    "Benchmark SQLite WAL checkpoint throughput",
    "Trim redundant Docker build layers",
    "Update OpenAPI schema for computer routes",
    "Fix terminal resize wrap on narrow panes",
    "Check TLS certificate pin mismatch warning",
    "Profile vector index quantization accuracy",
  ];
  const generated: RemoteSessionRowEntry[] = [...REMOTE_SESSION_ENTRIES];
  for (let i = 0; i < 24; i += 1) {
    const computer = scaleComputers[i % scaleComputers.length]!;
    const isOffline = computer.status === "offline";
    const project = computer.projects[0] ?? "api";
    const state: UnifiedSessionFixture["state"] = isOffline
      ? "idle"
      : i % 5 === 0
        ? "attention"
        : i % 3 === 0
          ? "running"
          : "done";
    const column: UnifiedSessionFixture["column"] =
      state === "attention"
        ? "needs_you"
        : state === "running"
          ? "in_progress"
          : i % 4 === 0
            ? "in_review"
            : "done";
    generated.push({
      id: `sess-vol-${i + 1}`,
      title: `${titles[i % titles.length]} (#${320 + i})`,
      computerId: computer.id,
      computerName: computer.name,
      isLocal: false,
      computerOffline: isOffline,
      project,
      agent: "general",
      model: i % 3 === 0 ? "Codex" : "Sonnet 4.6",
      provider: i % 3 === 0 ? "codex" : "claude",
      state,
      column,
      branch: `fix/batch-${320 + i}`,
      needsYouPhrase: state === "attention" ? "Needs approval" : undefined,
      updatedLabel: `${12 + i * 7}m`,
      minutesAgo: 12 + i * 7,
      iconKind: computer.iconKind,
      platform: computer.platform,
    });
  }
  return generated;
})();

export interface TranscriptTurnFixture {
  id: string;
  role: "user" | "agent";
  text: string;
  toolLine?: string;
}

export const DEVBOX_TRANSCRIPT: readonly TranscriptTurnFixture[] = [
  {
    id: "turn-1",
    role: "user",
    text: "Run the Postgres connection pool tests on devbox and fix whatever makes TestPoolExhaustion flake.",
  },
  {
    id: "turn-2",
    role: "agent",
    toolLine: "Ran `go test -race -count=20 ./internal/pgpool/...` on devbox (14.2s)",
    text: "Reproduced the flake on run 7 of 20. `Acquire` releases the semaphore before decrementing `inFlight`, so a concurrent `Close` observes a negative waiter count.\n\nI've updated `internal/pgpool/pool.go` to decrement `inFlight` before releasing the semaphore and I'm re-running the race suite now.",
  },
];

export const WORKSTATION_OFFLINE_TRANSCRIPT: readonly TranscriptTurnFixture[] = [
  {
    id: "ws-turn-1",
    role: "user",
    text: "Refactor the GPU batch scheduler so micro-batches pack by sequence length without padding stalls.",
  },
  {
    id: "ws-turn-2",
    role: "agent",
    toolLine: "Edited `trainer/scheduler/pack.py` on workstation · +84 −31",
    text: "Implemented bucketed length packing with a 256-token stride and verified zero gradient drift on the 1k-step smoke run.\n\nNext step when you're ready: run the full 8-GPU throughput benchmark and compare tokens/sec against baseline.",
  },
];

/** Computer with narrowed scopes (no `terminal:operate`) for Screen 06 U6 truthful-hiding state. */
export const DEVBOX_NARROWED_SCOPES: ComputerFixture = {
  ...COMPUTERS_PRO[2]!,
  scopes: ["sessions:operate", "files:read", "files:write"],
};
