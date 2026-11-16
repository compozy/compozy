/**
 * Session MCP fixtures in the wire shape of `GET /api/workspaces/{ws}/sessions/{id}/mcp`
 * (spec `mcp-extension-hot-activation`, `_s6-report.md`). Shared by component tests and the
 * `session-mcp-control` proto draft.
 */
import type {
  SessionMCPResponse,
  SessionMCPServerRow,
  SessionRuntimeMCPRow,
} from "../adapters/session-mcp-api";
import type { SessionMCPReloadView } from "../lib/session-mcp-reload-event";
import type { SessionMCPSummary } from "../lib/session-mcp-view";

const ATTENTION_STATES = new Set(["needs_sign_in", "waiting_approval", "failed"]);

export function sessionMCPRowFixture(
  partial: Partial<SessionMCPServerRow> & Pick<SessionMCPServerRow, "name">
): SessionMCPServerRow {
  const state = partial.state ?? "ready";
  const tools = partial.tool_count ?? (state === "ready" ? 8 : 0);
  return {
    owner: "",
    state,
    tool_count: tools,
    callable_count: state === "ready" ? tools : 0,
    reason: "",
    source: { kind: "global-config" },
    ...partial,
  };
}

const runtimeRow = (name: string, state = "loaded_by_runtime"): SessionRuntimeMCPRow => ({
  name,
  scope: "user",
  transport: "stdio",
  state,
});

const RUNTIME_SERVERS = [runtimeRow("context7"), runtimeRow("playwright"), runtimeRow("exa")];

const READY = [
  sessionMCPRowFixture({ name: "linear", tool_count: 23 }),
  sessionMCPRowFixture({ name: "github-local", tool_count: 12 }),
  sessionMCPRowFixture({ name: "grafana", tool_count: 8 }),
];

interface SessionMCPFixtureInput {
  compozy?: SessionMCPServerRow[];
  runtime?: string;
  runtimeDisplayName?: string;
  runtimeServers?: SessionRuntimeMCPRow[];
  runtimeMCP?: string;
  pendingReload?: boolean;
  lastReload?: SessionMCPResponse["last_reload"];
}

export function sessionMCPFixture(input: SessionMCPFixtureInput = {}): SessionMCPResponse {
  const compozy = input.compozy ?? READY;
  return {
    session_id: "sess-mcp",
    runtime: input.runtime ?? "claude",
    runtime_display_name: input.runtimeDisplayName ?? "Claude Code",
    usable: compozy.filter(row => row.state === "ready").length,
    attention: compozy.filter(row => ATTENTION_STATES.has(row.state)).length,
    pending_reload: input.pendingReload ?? false,
    last_reload: input.lastReload ?? null,
    compozy_servers: compozy,
    runtime_servers: {
      runtime_mcp: input.runtimeMCP ?? "load",
      servers: input.runtimeServers ?? RUNTIME_SERVERS,
    },
  };
}

/** The summary the session payload carries for this read model. */
export function sessionMCPSummaryFixture(data: SessionMCPResponse): SessionMCPSummary {
  return { usable: data.usable, attention: data.attention, pending_reload: data.pending_reload };
}

export const sessionMCPAllReadyFixture = sessionMCPFixture();

export const sessionMCPPendingFixture = sessionMCPFixture({
  pendingReload: true,
  compozy: [...READY, sessionMCPRowFixture({ name: "sentry", state: "applies_next_message" })],
});

export const sessionMCPNeedsSignInFixture = sessionMCPFixture({
  compozy: [sessionMCPRowFixture({ name: "notion", state: "needs_sign_in" }), ...READY],
});

export const sessionMCPWaitingFixture = sessionMCPFixture({
  compozy: [
    sessionMCPRowFixture({
      name: "sentry",
      owner: "",
      state: "waiting_approval",
      reason: "Added by a file in this project",
      scope: "workspace",
      workspace_id: "ws_project",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@sentry/mcp-server"],
      source: { kind: "workspace-mcp-sidecar", path: "/Users/dev/project/.mcp.json" },
      approval: { reason: "new", definition_digest: "sha256:sentry" },
    }),
    ...READY,
  ],
});

export const sessionMCPFailedFixture = sessionMCPFixture({
  compozy: [
    sessionMCPRowFixture({
      name: "pagerduty",
      state: "failed",
      reason: "The server closed the connection on start",
    }),
    ...READY,
  ],
});

export const sessionMCPReloadFailedFixture = sessionMCPFixture({
  pendingReload: true,
  lastReload: { at: "2026-11-14T16:30:00Z", outcome: "failed", reason: "fingerprint" },
  compozy: [...READY, sessionMCPRowFixture({ name: "sentry", state: "applies_next_message" })],
});

export const sessionMCPLimitedFixture = sessionMCPFixture({
  compozy: [
    READY[0]!,
    sessionMCPRowFixture({
      name: "github-local",
      state: "not_available",
      reason: "Limited by the parent session",
    }),
    sessionMCPRowFixture({
      name: "grafana",
      state: "not_available",
      reason: "Limited by the parent session",
    }),
  ],
});

export const sessionMCPRuntimeOffFixture = sessionMCPFixture({
  runtimeMCP: "off",
  runtimeServers: RUNTIME_SERVERS.map(row => ({ ...row, state: "off_in_compozy_sessions" })),
});

export const sessionMCPManyFixture = sessionMCPFixture({
  compozy: Array.from({ length: 50 }, (_, index) =>
    sessionMCPRowFixture({
      name: `server-${String(index + 1).padStart(2, "0")}`,
      state: index === 7 ? "needs_sign_in" : "ready",
      tool_count: (index % 9) + 2,
    })
  ),
});

export const sessionMCPCodexFixture = sessionMCPFixture({
  runtime: "codex",
  runtimeDisplayName: "Codex",
  runtimeServers: [runtimeRow("github"), runtimeRow("linear"), runtimeRow("local-scripts")],
});

export const sessionMCPReloadAppliedFixture: SessionMCPReloadView = {
  outcome: "applied",
  added: ["sentry"],
  removed: [],
};

export const sessionMCPReloadReplayedFixture: SessionMCPReloadView = {
  outcome: "replayed",
  added: ["sentry"],
  removed: [],
};

export const sessionMCPReloadFailedLineFixture: SessionMCPReloadView = {
  outcome: "failed",
  added: ["sentry"],
  removed: [],
  runtime: "Claude Code",
};
