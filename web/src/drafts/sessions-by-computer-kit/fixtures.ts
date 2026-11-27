/**
 * One operator, four computers and six projects: the same names as the landing page's lab, so
 * the board and the sidebar tell one story.
 */
import {
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
  makeScaleComputers,
  type ComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import type { SessionsBoardColumnView } from "@/systems/session/hooks/use-sessions-board";
import type {
  SessionBoardColumnId,
  SessionPullRequest,
} from "@/systems/session/lib/session-board-contract";
import type { SessionRowComputerInfo } from "@/systems/session/lib/session-list-computer";
import { primarySessionFixture, sessionRuntimeWithModel } from "@/systems/session/mocks/fixtures";
import { sessionsBoardViewFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import type { SessionPayload } from "@/systems/session/types";

export const LOCAL = LOCAL_COMPUTER;
export const STUDIO = COMPUTERS_PRO[1]!;
export const DEVBOX = COMPUTERS_PRO[2]!;
export const WORKSTATION: ComputerFixture = { ...COMPUTERS_PRO[4]!, lastConnectedLabel: "2h ago" };
export const REMOTES: readonly ComputerFixture[] = [STUDIO, DEVBOX, WORKSTATION];

export const PROJECTS: Readonly<Record<string, string>> = {
  ws_checkout: "checkout-web",
  ws_payments: "payments-platform",
  ws_risk: "risk-engine",
  ws_docs: "docs-site",
  ws_launch: "launch-hq",
  ws_infra: "platform-infra",
};

export function projectName(workspaceId: string | null | undefined): string {
  return (workspaceId && PROJECTS[workspaceId]) || "Unknown project";
}

const NOW = Date.now();
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();

function pr(number: number, overrides: Partial<SessionPullRequest> = {}): SessionPullRequest {
  return {
    provider: "github",
    number,
    url: `https://github.com/acme/payments/pull/${number}`,
    state: "open",
    draft: false,
    head_sha: "8f3c2a1d40",
    checks: { state: "passing", passed: 12, failed: 0, pending: 0, failed_names: [] },
    review_decision: "review_required",
    mergeable: "mergeable",
    ready_to_merge: false,
    ...overrides,
  };
}

export interface ComputerSession {
  session: SessionPayload;
  computerId: string;
}

function entry(
  computerId: string,
  id: string,
  column: SessionBoardColumnId,
  workspaceId: string,
  name: string,
  overrides: Partial<SessionPayload> & { provider?: string; minutes?: number } = {}
): ComputerSession {
  const { provider = "claude", minutes = 5, ...rest } = overrides;
  return {
    computerId,
    session: {
      ...primarySessionFixture,
      id,
      name,
      workspace_id: workspaceId,
      agent_name: provider,
      runtime: sessionRuntimeWithModel(provider),
      state: "active",
      badge: "idle",
      board_column: column,
      archived_at: null,
      updated_at: ago(minutes),
      pull_request: null,
      pull_request_lookup: rest.branch
        ? { state: "ok", cause: "", observed_at: ago(1), stale: false }
        : null,
      ...rest,
    },
  };
}

const PENDING = { state: "pending", passed: 8, failed: 0, pending: 4, failed_names: [] } as const;
const FAILING = { state: "failing", passed: 9, failed: 2, pending: 0, failed_names: ["lint"] };

export const ENTRIES: readonly ComputerSession[] = [
  entry("local", "l1", "needs_you", "ws_checkout", "Raise the Brazil canary to 50%", {
    badge: "waiting-for-input",
    minutes: 2,
  }),
  entry("local", "l2", "working", "ws_launch", "Draft the Q4 dispute evidence playbook", {
    badge: "running",
    minutes: 0,
  }),
  entry("local", "l3", "in_review", "ws_checkout", "Ship Brazil PIX refunds", {
    badge: "done",
    branch: "feat/pix-refunds",
    pull_request: pr(485),
    provider: "codex",
    minutes: 21,
  }),
  entry("local", "l4", "done", "ws_docs", "Screenshot the new receipt pages for the docs", {
    badge: "done",
    minutes: 9,
  }),
  entry("local", "l5", "done", "ws_payments", "Review the ledger v2 schema diff", {
    badge: "done",
    provider: "codex",
    minutes: 48,
  }),
  entry(STUDIO.id, "st1", "needs_you", "ws_checkout", "Record the Pix checkout walkthrough", {
    badge: "waiting-for-input",
    minutes: 1,
  }),
  entry(STUDIO.id, "st2", "working", "ws_docs", "Rebuild the receipts docs preview", {
    badge: "running",
    provider: "codex",
    minutes: 0,
  }),
  entry(STUDIO.id, "st3", "waiting", "ws_checkout", "Watch CI on the installment selector", {
    badge: "waiting",
    waiting_on: "monitor",
    branch: "feat/installment-tokens",
    pull_request: pr(491, { checks: PENDING as SessionPullRequest["checks"] }),
    minutes: 3,
  }),
  entry(STUDIO.id, "st4", "done", "ws_checkout", "Migrate the installment selector to tokens", {
    badge: "done",
    minutes: 35,
  }),
  entry(DEVBOX.id, "d1", "needs_you", "ws_payments", "Pick the retry policy for SPEI webhooks", {
    badge: "waiting-for-input",
    provider: "codex",
    minutes: 0,
  }),
  entry(DEVBOX.id, "d2", "working", "ws_payments", "Dry-run the ledger v2 migration", {
    badge: "running",
    provider: "codex",
    minutes: 0,
  }),
  entry(DEVBOX.id, "d3", "working", "ws_risk", "Replay October settlements on the full dataset", {
    badge: "running",
    minutes: 0,
  }),
  entry(DEVBOX.id, "d4", "in_review", "ws_payments", "Cap settlement retries per partner window", {
    badge: "done",
    branch: "fix/settlement-retry-budget",
    pull_request: pr(482, { review_decision: "approved", ready_to_merge: true }),
    minutes: 120,
  }),
  entry(DEVBOX.id, "d5", "needs_you", "ws_risk", "Backtest the dispute model on October", {
    badge: "failed",
    minutes: 27,
  }),
  entry(DEVBOX.id, "d6", "done", "ws_infra", "Vacuum the settlement archive", {
    badge: "done",
    provider: "codex",
    minutes: 9,
  }),
  entry(WORKSTATION.id, "w1", "working", "ws_risk", "Rebuild the card velocity feature store", {
    badge: "idle",
    minutes: 130,
  }),
  entry(WORKSTATION.id, "w2", "in_review", "ws_infra", "Rotate PSP webhook signing secrets", {
    badge: "done",
    branch: "chore/rotate-psp-secrets",
    pull_request: pr(479, { checks: FAILING as SessionPullRequest["checks"] }),
    minutes: 240,
  }),
];

/** What every remote row and card reads for its computer label; local rows carry none. */
export function computerInfoMap(
  entries: readonly ComputerSession[],
  computers: readonly ComputerFixture[] = REMOTES
): ReadonlyMap<string, SessionRowComputerInfo> {
  const byId = new Map(computers.map(computer => [computer.id, computer]));
  const map = new Map<string, SessionRowComputerInfo>();
  for (const item of entries) {
    const computer = byId.get(item.computerId);
    if (!computer) continue;
    map.set(item.session.id, {
      id: computer.id,
      name: computer.name,
      platform: computer.platform,
      iconKind: computer.iconKind,
      offline: computer.status === "offline",
    });
  }
  return map;
}

export function sessionsOn(computerId: string, entries: readonly ComputerSession[] = ENTRIES) {
  return entries.flatMap(item => (item.computerId === computerId ? [item.session] : []));
}

/** The board's five columns over a set of sessions, every count the rows on show. */
export function columnsOf(sessions: readonly SessionPayload[]): SessionsBoardColumnView[] {
  return sessionsBoardViewFixture().columns.map(column => {
    const rows = sessions.filter(session => session.board_column === column.id);
    return { ...column, sessions: rows, count: rows.length, hasMore: false };
  });
}

const SCALE_PROJECTS = Object.keys(PROJECTS);
const SCALE_TITLES = [
  "Profile the checkout bundle",
  "Retry flaky payout webhooks",
  "Index the dispute archive",
  "Audit the PSP key rotation",
  "Backfill chargeback reasons",
  "Tune the fraud score threshold",
];

/** Eight remote computers with a few sessions each, for the volume state. */
export const SCALE_COMPUTERS: readonly ComputerFixture[] = [
  STUDIO,
  DEVBOX,
  WORKSTATION,
  // The scale helper starts from the seven Pro fixtures; its next five are generated hosts,
  // renamed here so every id and name is unique.
  ...makeScaleComputers(COMPUTERS_PRO.length + 5)
    .slice(COMPUTERS_PRO.length)
    .map((computer, index) => ({
      ...computer,
      id: `cmp_scale_${index}`,
      name: ["ci-runner-01", "mac-mini-02", "staging-eu", "build-farm-03", "qa-linux-04"][index]!,
    })),
];

export const SCALE_ENTRIES: readonly ComputerSession[] = [
  ...ENTRIES,
  ...SCALE_COMPUTERS.slice(3).flatMap((computer, index) =>
    Array.from({ length: 2 + (index % 3) }, (_, row) =>
      entry(
        computer.id,
        `${computer.id}-${row}`,
        (["working", "needs_you", "in_review", "done"] as const)[(index + row) % 4]!,
        SCALE_PROJECTS[(index + row) % SCALE_PROJECTS.length]!,
        SCALE_TITLES[(index * 2 + row) % SCALE_TITLES.length]!,
        {
          badge: (["running", "waiting-for-input", "done", "done"] as const)[(index + row) % 4]!,
          provider: row % 2 === 0 ? "claude" : "codex",
          minutes: 4 + index * 7 + row * 3,
        }
      )
    )
  ),
];

/** A session folder: one computer, one project, one profile. */
export interface SessionFolderFixture {
  id: string;
  name: string;
  computerId: string;
  workspaceId: string;
  sessionIds: readonly string[];
}

/** A project folder: one computer, one profile. */
export interface ProjectFolderFixture {
  id: string;
  name: string;
  computerId: string;
  workspaceIds: readonly string[];
}

// "PIX launch" exists on this computer and on studio: equal names, two folders, never merged.
export const SESSION_FOLDERS: readonly SessionFolderFixture[] = [
  {
    id: "f_pix_local",
    name: "PIX launch",
    computerId: "local",
    workspaceId: "ws_checkout",
    sessionIds: ["l1", "l3"],
  },
  {
    id: "f_pix_studio",
    name: "PIX launch",
    computerId: STUDIO.id,
    workspaceId: "ws_checkout",
    sessionIds: ["st1", "st3"],
  },
  {
    id: "f_ledger",
    name: "Ledger v2",
    computerId: DEVBOX.id,
    workspaceId: "ws_payments",
    sessionIds: ["d2", "d4"],
  },
  {
    id: "f_spei",
    name: "SPEI webhooks",
    computerId: DEVBOX.id,
    workspaceId: "ws_payments",
    sessionIds: [],
  },
];

export const PROJECT_FOLDERS: readonly ProjectFolderFixture[] = [
  {
    id: "pf_money",
    name: "Money movement",
    computerId: DEVBOX.id,
    workspaceIds: ["ws_payments", "ws_risk"],
  },
];

/** The computers of the states view: one per way a computer can fail to list its sessions. */
export const HETZNER = COMPUTERS_PRO[3]!;
export const GPU_VM = COMPUTERS_PRO[5]!;
export const MINI_BUILD: ComputerFixture = {
  ...COMPUTERS_PRO[0]!,
  status: "needs_pro",
  planState: "limited",
  routeInUse: null,
  load: null,
};
export const HOMELAB: ComputerFixture = {
  ...WORKSTATION,
  id: "cmp_2d7n4k6v",
  name: "homelab",
  platform: "linux_arm64",
  iconKind: "server",
  lastConnectedLabel: "12d ago",
};
