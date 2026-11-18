/**
 * Fixtures for the composer MCP redesign, in the wire shape of
 * `GET /api/workspaces/{ws}/sessions/{id}/mcp`. The default mirrors the reported screenshot:
 * one CompozyOS server that needs sign-in and nine servers Claude Code loads on its own.
 */
import type { SessionRuntimeMCPRow } from "@/systems/session/adapters/session-mcp-api";
import {
  sessionMCPFixture,
  sessionMCPRowFixture,
} from "@/systems/session/mocks/session-mcp-fixtures";

const runtimeRow = (
  name: string,
  state = "loaded_by_runtime",
  transport = "stdio"
): SessionRuntimeMCPRow => ({ name, scope: "user", transport, state });

export const CLAUDE_SERVERS: SessionRuntimeMCPRow[] = [
  runtimeRow("DaVinci Resolve"),
  runtimeRow("Neon", "loaded_by_runtime", "http"),
  runtimeRow("advos", "loaded_by_runtime", "http"),
  runtimeRow("branas", "loaded_by_runtime", "http"),
  runtimeRow("cap"),
  runtimeRow("cua-driver"),
  runtimeRow("higgsfield", "loaded_by_runtime", "http"),
  runtimeRow("inspo", "loaded_by_runtime", "http"),
  runtimeRow("linear-server", "loaded_by_runtime", "http"),
];

const READY = [
  sessionMCPRowFixture({ name: "linear", tool_count: 23 }),
  sessionMCPRowFixture({ name: "github", tool_count: 41 }),
  sessionMCPRowFixture({ name: "sentry", tool_count: 12 }),
];

const SIGN_IN = sessionMCPRowFixture({
  name: "branas-dashboard",
  state: "needs_sign_in",
  reason: "Sign in to branas-dashboard in CompozyOS",
});

/** The screenshot: one server needs sign-in, nine Claude Code servers. */
export const signInFixture = sessionMCPFixture({
  compozy: [SIGN_IN, ...READY],
  runtimeServers: CLAUDE_SERVERS,
});

/** Same session right after a successful sign-in: the server joins on the next message. */
export const signedInFixture = sessionMCPFixture({
  pendingReload: true,
  compozy: [
    sessionMCPRowFixture({ name: "branas-dashboard", state: "applies_next_message" }),
    ...READY,
  ],
  runtimeServers: CLAUDE_SERVERS,
});

export const allReadyFixture = sessionMCPFixture({
  compozy: READY,
  runtimeServers: CLAUDE_SERVERS,
});

export const waitingFixture = sessionMCPFixture({
  compozy: [
    sessionMCPRowFixture({
      name: "postgres-local",
      state: "waiting_approval",
      reason: "Added by a file in this project",
    }),
    ...READY,
  ],
  runtimeServers: CLAUDE_SERVERS,
});

export const failedFixture = sessionMCPFixture({
  compozy: [
    sessionMCPRowFixture({
      name: "pagerduty",
      state: "failed",
      reason: "No callable tools available",
    }),
    ...READY,
  ],
  runtimeServers: CLAUDE_SERVERS,
});

export const reloadFailedFixture = sessionMCPFixture({
  pendingReload: true,
  lastReload: { at: "2026-11-16T12:30:00Z", outcome: "failed", reason: "fingerprint" },
  compozy: [
    sessionMCPRowFixture({ name: "branas-dashboard", state: "applies_next_message" }),
    ...READY,
  ],
  runtimeServers: CLAUDE_SERVERS,
});

export const limitedFixture = sessionMCPFixture({
  compozy: [
    READY[0]!,
    sessionMCPRowFixture({
      name: "github",
      state: "not_available",
      reason: "Limited by the parent session",
    }),
    sessionMCPRowFixture({
      name: "sentry",
      state: "not_available",
      reason: "Limited by the parent session",
    }),
  ],
  runtimeServers: CLAUDE_SERVERS,
});

export const runtimeMixedFixture = sessionMCPFixture({
  compozy: READY,
  runtimeServers: [
    ...CLAUDE_SERVERS.slice(0, 6),
    runtimeRow("figma", "waiting_in_runtime", "http"),
    runtimeRow("vercel", "off_in_runtime", "http"),
  ],
});

export const runtimeOffFixture = sessionMCPFixture({
  compozy: READY,
  runtimeMCP: "off",
  runtimeServers: CLAUDE_SERVERS.map(row => ({ ...row, state: "off_in_compozy_sessions" })),
});

export const manyFixture = sessionMCPFixture({
  compozy: Array.from({ length: 50 }, (_, index) =>
    sessionMCPRowFixture({
      name: `server-${String(index + 1).padStart(2, "0")}`,
      state: index === 7 ? "needs_sign_in" : "ready",
      tool_count: (index % 9) + 2,
    })
  ),
  runtimeServers: CLAUDE_SERVERS,
});

export const codexFixture = sessionMCPFixture({
  runtime: "codex",
  runtimeDisplayName: "Codex",
  compozy: [SIGN_IN, ...READY],
  runtimeServers: [runtimeRow("github"), runtimeRow("linear"), runtimeRow("local-scripts")],
});

export const emptyRuntimeFixture = sessionMCPFixture({
  compozy: [SIGN_IN, ...READY],
  runtimeServers: [],
});
