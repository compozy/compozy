import type {
  SessionProcessGroupPayload,
  SessionProcessPayload,
} from "../adapters/session-processes-api";
import {
  processIdentity,
  singleProcessTrees,
  type SessionProcessTree,
} from "../lib/session-processes";
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

/** `sessionProcessesFixture` as rows: one tree per process. */
export const sessionProcessTreesFixture: readonly SessionProcessTree[] =
  singleProcessTrees(sessionProcessesFixture);

const CHROME = "/Applications/Google Chrome.app/Contents";
const CHROME_HELPERS = `${CHROME}/Frameworks/Google Chrome Framework.framework/Versions/142.0.7444.60/Helpers`;
const CHROME_PROFILE = "--user-data-dir=/var/folders/tq/T/agent-browser-chrome-4e1f";

function chromeHelper(
  pid: number,
  program: string,
  type: string,
  extra = ""
): SessionProcessPayload {
  return sessionProcessFixture({
    pid,
    pgid: 81020,
    started_at: minutesAgo(23),
    command: `${CHROME_HELPERS}/${program}.app/Contents/MacOS/${program} --type=${type} ${CHROME_PROFILE}${extra}`,
    argv0: `${CHROME_HELPERS}/${program}.app/Contents/MacOS/${program}`,
    cwd: "/",
    ports: [],
    age_seconds: 23 * 60,
  });
}

function crashpad(pid: number): SessionProcessPayload {
  return sessionProcessFixture({
    pid,
    pgid: pid,
    started_at: minutesAgo(23),
    command: `${CHROME_HELPERS}/chrome_crashpad_handler --monitor-self-annotation=ptype=crashpad-handler ${CHROME_PROFILE}`,
    argv0: `${CHROME_HELPERS}/chrome_crashpad_handler`,
    cwd: "/",
    ports: [],
    age_seconds: 23 * 60,
  });
}

/**
 * Issue #1100's session: a gallery dev server in a Terminal and the headless browser
 * `agent-browser` left behind, 13 processes in all.
 */
const galleryTerminal = sessionProcessFixture({
  pid: 80412,
  pgid: 80412,
  started_at: minutesAgo(2),
  kind: "terminal",
  command: "bun run proto dev",
  argv0: "bun",
  cwd: "/Users/dev/acme",
  ports: [4400, 4401],
  age_seconds: 2 * 60,
  terminal_id: "term-gallery",
});
const browserDaemon = sessionProcessFixture({
  pid: 81002,
  pgid: 81002,
  started_at: minutesAgo(23),
  command: "node /Users/dev/.agent-browser/dist/daemon.js --session default",
  argv0: "node",
  cwd: "/Users/dev/acme",
  ports: [],
  age_seconds: 23 * 60,
});
const chromeMain = sessionProcessFixture({
  pid: 81020,
  pgid: 81020,
  started_at: minutesAgo(23),
  command: `${CHROME}/MacOS/Google Chrome --headless=new --remote-debugging-port=9222 ${CHROME_PROFILE}`,
  argv0: `${CHROME}/MacOS/Google Chrome`,
  cwd: "/",
  ports: [9222],
  age_seconds: 23 * 60,
});
const chromeHelpers = [
  chromeHelper(81031, "Google Chrome Helper (GPU)", "gpu-process"),
  chromeHelper(
    81032,
    "Google Chrome Helper",
    "utility",
    " --utility-sub-type=network.mojom.NetworkService"
  ),
  chromeHelper(
    81033,
    "Google Chrome Helper",
    "utility",
    " --utility-sub-type=storage.mojom.StorageService"
  ),
  chromeHelper(81040, "Google Chrome Helper (Renderer)", "renderer"),
  chromeHelper(81041, "Google Chrome Helper (Renderer)", "renderer"),
  chromeHelper(81042, "Google Chrome Helper (Renderer)", "renderer"),
  chromeHelper(81043, "Google Chrome Helper (Renderer)", "renderer"),
  chromeHelper(81044, "Google Chrome Helper (Renderer)", "renderer"),
];
const browserTree: SessionProcessTree = {
  id: processIdentity(browserDaemon),
  kind: "background",
  lead: browserDaemon,
  wrappers: [],
  helpers: [chromeMain, ...chromeHelpers, crashpad(81011), crashpad(81013)],
  ports: [9222],
};
const galleryTree: SessionProcessTree = singleProcessTrees([galleryTerminal])[0]!;

/** The flat list issue #1100 reported: 13 rows for a Terminal and a browser. */
export const issue1100FlatTreesFixture: readonly SessionProcessTree[] = singleProcessTrees([
  galleryTerminal,
  browserDaemon,
  crashpad(81011),
  crashpad(81013),
  chromeMain,
  ...chromeHelpers,
]);

/** The same session grouped: one row per logical tree. */
export const issue1100GroupedTreesFixture: readonly SessionProcessTree[] = [
  galleryTree,
  browserTree,
];

const webStart = sessionProcessFixture({
  pid: 82100,
  pgid: 82100,
  started_at: minutesAgo(9),
  command: "bun run --cwd apps/web start",
  argv0: "bun",
  cwd: "/Users/dev/acme",
  ports: [],
  age_seconds: 9 * 60,
});
const webServer = sessionProcessFixture({
  pid: 82104,
  pgid: 82100,
  started_at: minutesAgo(9),
  command: "node .output/server/index.mjs",
  argv0: "node",
  cwd: "/Users/dev/acme/apps/web",
  ports: [3000],
  age_seconds: 9 * 60,
});
const protoOuter = sessionProcessFixture({
  pid: 82200,
  pgid: 82200,
  started_at: minutesAgo(4),
  command: "bun run proto dev",
  argv0: "bun",
  cwd: "/Users/dev/acme",
  ports: [],
  age_seconds: 4 * 60,
});
const protoInner = sessionProcessFixture({
  pid: 82203,
  pgid: 82200,
  started_at: minutesAgo(4),
  command: "bun run --cwd web proto dev",
  argv0: "bun",
  cwd: "/Users/dev/acme",
  ports: [],
  age_seconds: 4 * 60,
});
const protoLeaf = sessionProcessFixture({
  pid: 82207,
  pgid: 82200,
  started_at: minutesAgo(4),
  command: "node /Users/dev/acme/web/node_modules/.bin/proto dev",
  argv0: "node",
  cwd: "/Users/dev/acme/web",
  ports: [4400, 4401],
  age_seconds: 4 * 60,
});

/** Wrapper chains: `bun run` → `node`, named by the server they wrap. */
export const wrapperChainTreesFixture: readonly SessionProcessTree[] = [
  {
    id: processIdentity(protoOuter),
    kind: "background",
    lead: protoLeaf,
    wrappers: [protoOuter, protoInner],
    helpers: [],
    ports: [4400, 4401],
  },
  {
    id: processIdentity(webStart),
    kind: "background",
    lead: webServer,
    wrappers: [webStart],
    helpers: [],
    ports: [3000],
  },
];

const toolCall = sessionProcessFixture({
  pid: 83300,
  pgid: 83300,
  started_at: minutesAgo(0.5),
  kind: "tool",
  command: "go test ./internal/sessionprocess/...",
  argv0: "go",
  cwd: "/Users/dev/acme",
  ports: [],
  age_seconds: 30,
});
const toolCompile = sessionProcessFixture({
  pid: 83307,
  pgid: 83300,
  started_at: minutesAgo(0.4),
  kind: "tool",
  command: "/usr/local/go/pkg/tool/darwin_arm64/compile -o /tmp/go-build/b001/_pkg_.a",
  argv0: "/usr/local/go/pkg/tool/darwin_arm64/compile",
  cwd: "/Users/dev/acme",
  ports: [],
  age_seconds: 24,
});

/** Left running plus a tool call still running in the agent's turn. */
export const mixedToolTreesFixture: readonly SessionProcessTree[] = [
  galleryTree,
  browserTree,
  {
    id: processIdentity(toolCall),
    kind: "tool",
    lead: toolCall,
    wrappers: [],
    helpers: [toolCompile],
    ports: [],
  },
];
