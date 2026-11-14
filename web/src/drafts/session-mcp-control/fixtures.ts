/**
 * Session MCP read model fixtures in the shape of `GET /api/workspaces/{ws}/sessions/{id}/mcp`
 * (spec `mcp-extension-hot-activation`, `_dx.md`). One state word per server, shared with the CLI
 * and the native tool.
 */
export type SessionMCPState =
  | "ready"
  | "applies_next_message"
  | "needs_sign_in"
  | "waiting_approval"
  | "failed"
  | "not_available"
  | "loaded_by_runtime"
  | "off_in_compozy_sessions";

export interface SessionMCPServer {
  name: string;
  state: SessionMCPState;
  tools?: number;
  reason?: string;
}

export interface SessionMCPModel {
  usable: number;
  attention: number;
  pendingReload: boolean;
  reloadFailed: boolean;
  stale: boolean;
  runtime: { id: string; displayName: string; servers: readonly SessionMCPServer[] };
  compozy: readonly SessionMCPServer[];
  unavailable: readonly SessionMCPServer[];
}

const RUNTIME_SERVERS: readonly SessionMCPServer[] = [
  { name: "context7", state: "loaded_by_runtime" },
  { name: "playwright", state: "loaded_by_runtime" },
  { name: "exa", state: "loaded_by_runtime" },
];

const READY: readonly SessionMCPServer[] = [
  { name: "linear", state: "ready", tools: 23 },
  { name: "github-local", state: "ready", tools: 12 },
  { name: "grafana", state: "ready", tools: 8 },
];

function model(partial: Partial<SessionMCPModel>): SessionMCPModel {
  const compozy = partial.compozy ?? READY;
  return {
    usable: compozy.filter(server => server.state === "ready").length,
    attention: compozy.filter(server =>
      ["needs_sign_in", "waiting_approval", "failed"].includes(server.state)
    ).length,
    pendingReload: false,
    reloadFailed: false,
    stale: false,
    runtime: { id: "claude", displayName: "Claude Code", servers: RUNTIME_SERVERS },
    compozy,
    unavailable: [],
    ...partial,
  };
}

export const ALL_READY = model({});

export const PENDING = model({
  pendingReload: true,
  compozy: [...READY, { name: "sentry", state: "applies_next_message" }],
});

export const NEEDS_SIGN_IN = model({
  compozy: [{ name: "notion", state: "needs_sign_in" }, ...READY],
});

export const WAITING = model({
  compozy: [
    { name: "sentry", state: "waiting_approval", reason: "Added by a file in this project" },
    ...READY,
  ],
});

export const FAILED = model({
  compozy: [
    { name: "pagerduty", state: "failed", reason: "The server closed the connection on start" },
    ...READY,
  ],
});

export const RELOAD_FAILED = model({
  reloadFailed: true,
  compozy: [...READY, { name: "sentry", state: "applies_next_message" }],
});

export const LIMITED = model({
  compozy: [READY[0]!],
  unavailable: [
    { name: "github-local", state: "not_available", reason: "Limited by the parent session" },
    { name: "grafana", state: "not_available", reason: "Limited by the parent session" },
  ],
});

export const RUNTIME_OFF = model({
  runtime: {
    id: "claude",
    displayName: "Claude Code",
    servers: RUNTIME_SERVERS.map(server => ({ ...server, state: "off_in_compozy_sessions" })),
  },
});

export const STALE = model({ stale: true });

export const MANY = model({
  compozy: Array.from({ length: 50 }, (_, index) => ({
    name: `server-${String(index + 1).padStart(2, "0")}`,
    state: index === 7 ? "needs_sign_in" : "ready",
    tools: (index % 9) + 2,
  })) as SessionMCPServer[],
});

export const CODEX_SESSION = model({
  runtime: {
    id: "codex",
    displayName: "Codex",
    servers: [
      { name: "github", state: "loaded_by_runtime" },
      { name: "linear", state: "loaded_by_runtime" },
      { name: "local-scripts", state: "loaded_by_runtime" },
    ],
  },
});
