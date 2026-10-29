import type {
  SessionProcessGroupPayload,
  SessionProcessPayload,
} from "../adapters/session-processes-api";
import type { SessionPayload } from "../types";

const minutesAgo = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();

/** One running process row; overrides pick the kind, command, and ports. */
export function sessionProcessFixture(
  overrides: Partial<SessionProcessPayload> = {}
): SessionProcessPayload {
  return {
    pid: 76050,
    pgid: 76050,
    started_at: minutesAgo(42),
    kind: "background",
    command: "bun run dev --port 5173",
    argv0: "bun",
    cwd: "/Users/dev/acme/web",
    ports: [5173],
    age_seconds: 42 * 60,
    parent_alive: false,
    attribution: "env",
    ...overrides,
  };
}

/** A dev server, a detached watcher, and a Terminal: what a session typically leaves behind. */
export const sessionProcessesFixture: readonly SessionProcessPayload[] = [
  sessionProcessFixture(),
  sessionProcessFixture({
    pid: 76112,
    pgid: 76112,
    started_at: minutesAgo(18),
    command: "tsc --watch --preserveWatchOutput",
    argv0: "tsc",
    ports: [],
    age_seconds: 18 * 60,
    // First seen under the live agent; its environment is not readable.
    attribution: "lineage",
  }),
  sessionProcessFixture({
    pid: 77001,
    pgid: 77001,
    started_at: minutesAgo(6),
    kind: "terminal",
    command: "go run ./cmd/api",
    argv0: "go",
    cwd: "/Users/dev/acme/api",
    ports: [8080, 9090],
    age_seconds: 6 * 60,
    terminal_id: "term-api",
  }),
];

/** The summary the session payload carries for `sessionProcessesFixture`. */
export function withSessionProcessSummary(
  session: SessionPayload,
  processes: readonly SessionProcessPayload[] = sessionProcessesFixture
): SessionPayload {
  const background = processes.filter(process => process.kind === "background").length;
  const terminals = processes.filter(process => process.kind === "terminal").length;
  return {
    ...session,
    process_summary: {
      supported: true,
      background,
      terminals,
      total: processes.length,
      ports: processes.flatMap(process => process.ports),
    },
  };
}

/** "Left running": one archived session's dev servers and one deleted session's watcher. */
export const processesLeftRunningFixture: readonly SessionProcessGroupPayload[] = [
  {
    session_id: "sess-d93c3bbd6d69855f",
    workspace_id: "ws_alpha",
    profile: "default",
    session_name: "Landing page polish",
    orphaned: true,
    items: [
      sessionProcessFixture({
        pid: 76050,
        started_at: minutesAgo(7 * 24 * 60),
        command: "astro dev --port 4321",
        argv0: "astro",
        ports: [4321],
        age_seconds: 7 * 24 * 3600,
      }),
      sessionProcessFixture({
        pid: 76090,
        started_at: minutesAgo(7 * 24 * 60 - 5),
        command: "bun run storybook",
        argv0: "bun",
        ports: [6006],
        age_seconds: 7 * 24 * 3600 - 300,
      }),
    ],
  },
  {
    session_id: "sess-0b1e5c2a9f7d4e31",
    workspace_id: "ws_alpha",
    profile: "default",
    session_name: "",
    orphaned: true,
    items: [
      sessionProcessFixture({
        pid: 80211,
        started_at: minutesAgo(3 * 60),
        command: "vitest --watch",
        argv0: "vitest",
        ports: [],
        age_seconds: 3 * 3600,
      }),
    ],
  },
];
