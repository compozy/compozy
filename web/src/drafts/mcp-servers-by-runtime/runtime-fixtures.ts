/**
 * Runtime inventory fixtures in the shape of `GET /api/settings/runtime-mcp-servers`
 * (spec `mcp-extension-hot-activation`, `_dx.md`). Names only: the inventory never carries values.
 */
export type RuntimeId =
  | "claude"
  | "codex"
  | "gemini"
  | "opencode"
  | "kimi-cli"
  | "qwen-code"
  | "copilot"
  | "cursor"
  | "agy"
  | "pi";

export interface RuntimeServer {
  name: string;
  scope: "user" | "local" | "project" | "plugin";
  transport: "stdio" | "http" | "sse" | "ws";
  /** Command for stdio, URL host for remote servers. */
  target: string;
  sourceLabel: string;
  enabledInRuntime: boolean;
  approval: "not_required" | "approved" | "pending";
  auth: "unknown" | "signed_in" | "needs_sign_in" | "not_required";
  envKeys: readonly string[];
  headerKeys: readonly string[];
  alsoInCompozy: boolean;
  copyable: boolean;
  copyBlockedReason?: string;
}

export interface RuntimeGroupData {
  runtime: RuntimeId;
  displayName: string;
  installed: boolean;
  runtimeMcp: { mode: "load" | "off"; canDisable: boolean };
  readState: "ok" | "unreadable";
  readError?: string;
  configFiles: readonly string[];
  notes: readonly string[];
  servers: readonly RuntimeServer[];
}

function server(partial: Partial<RuntimeServer> & Pick<RuntimeServer, "name">): RuntimeServer {
  return {
    scope: "user",
    transport: "stdio",
    target: "npx",
    sourceLabel: "~/.claude.json",
    enabledInRuntime: true,
    approval: "not_required",
    auth: "unknown",
    envKeys: [],
    headerKeys: [],
    alsoInCompozy: false,
    copyable: true,
    ...partial,
  };
}

export const CLAUDE: RuntimeGroupData = {
  runtime: "claude",
  displayName: "Claude Code",
  installed: true,
  runtimeMcp: { mode: "load", canDisable: true },
  readState: "ok",
  configFiles: ["~/.claude.json", "~/Dev/platform/.mcp.json"],
  notes: ["claude.ai connectors aren't listed here. They show up inside sessions."],
  servers: [
    server({ name: "context7", target: "npx -y @upstash/context7-mcp" }),
    server({ name: "playwright", target: "npx @playwright/mcp@latest" }),
    server({
      name: "linear",
      transport: "http",
      target: "mcp.linear.app",
      alsoInCompozy: true,
    }),
    server({
      name: "exa",
      transport: "http",
      target: "mcp.exa.ai",
      headerKeys: ["Authorization"],
    }),
    server({
      name: "postgres",
      scope: "local",
      target: "uvx postgres-mcp",
      envKeys: ["DATABASE_URL"],
      sourceLabel: "~/.claude.json · this project",
    }),
    server({
      name: "sentry",
      scope: "project",
      transport: "http",
      target: "mcp.sentry.dev",
      approval: "pending",
      sourceLabel: "~/Dev/platform/.mcp.json",
    }),
    server({
      name: "notion",
      transport: "http",
      target: "mcp.notion.com",
      enabledInRuntime: false,
    }),
    server({
      name: "figma",
      scope: "plugin",
      transport: "http",
      target: "mcp.figma.com",
      sourceLabel: "figma plugin",
      copyable: false,
      copyBlockedReason: "Plugin servers stay in Claude Code",
    }),
  ],
};

export const CODEX: RuntimeGroupData = {
  runtime: "codex",
  displayName: "Codex",
  installed: true,
  runtimeMcp: { mode: "load", canDisable: false },
  readState: "ok",
  configFiles: ["~/.codex/config.toml"],
  notes: [],
  servers: [
    server({
      name: "github",
      target: "github-mcp-server stdio",
      envKeys: ["GITHUB_PERSONAL_ACCESS_TOKEN"],
      auth: "not_required",
      sourceLabel: "~/.codex/config.toml",
    }),
    server({
      name: "linear",
      transport: "http",
      target: "mcp.linear.app",
      alsoInCompozy: true,
      sourceLabel: "~/.codex/config.toml",
    }),
    server({
      name: "local-scripts",
      target: "node ./scripts/mcp.js",
      sourceLabel: "~/.codex/config.toml",
      copyable: false,
      copyBlockedReason: "Runs in its own folder",
    }),
    server({
      name: "computer-use",
      target: "codex-computer-use",
      enabledInRuntime: false,
      auth: "not_required",
      sourceLabel: "~/.codex/config.toml",
    }),
  ],
};

export const PI: RuntimeGroupData = {
  runtime: "pi",
  displayName: "Pi",
  installed: true,
  runtimeMcp: { mode: "load", canDisable: false },
  readState: "ok",
  configFiles: ["~/.pi/agent/mcp.json"],
  notes: [],
  servers: [
    server({
      name: "playwright",
      target: "npx @playwright/mcp@latest",
      sourceLabel: "~/.pi/agent/mcp.json",
    }),
  ],
};

export const GEMINI: RuntimeGroupData = {
  runtime: "gemini",
  displayName: "Gemini CLI",
  installed: false,
  runtimeMcp: { mode: "load", canDisable: false },
  readState: "ok",
  configFiles: ["~/.gemini/settings.json"],
  notes: [],
  servers: [
    server({
      name: "firebase",
      target: "npx -y firebase-tools mcp",
      sourceLabel: "~/.gemini/settings.json",
    }),
    server({
      name: "exa",
      transport: "http",
      target: "mcp.exa.ai",
      headerKeys: ["x-api-key"],
      sourceLabel: "~/.gemini/settings.json",
    }),
  ],
};

export const OPENCODE_UNREADABLE: RuntimeGroupData = {
  runtime: "opencode",
  displayName: "OpenCode",
  installed: true,
  runtimeMcp: { mode: "load", canDisable: false },
  readState: "unreadable",
  readError: "Unexpected token at line 14",
  configFiles: ["~/.config/opencode/opencode.jsonc"],
  notes: [],
  servers: [],
};

const CURSOR_NAMES = [
  "atlassian",
  "browserbase",
  "cloudflare",
  "datadog",
  "docker",
  "firecrawl",
  "gitlab",
  "grafana",
  "hubspot",
  "jira",
  "mongodb",
  "neon",
  "pagerduty",
  "pinecone",
  "redis",
  "sentry",
  "slack",
  "stripe",
  "supabase",
  "terraform",
  "twilio",
  "vercel",
  "zapier",
  "zendesk",
];

export const CURSOR_MANY: RuntimeGroupData = {
  runtime: "cursor",
  displayName: "Cursor Agent",
  installed: true,
  runtimeMcp: { mode: "load", canDisable: false },
  readState: "ok",
  configFiles: ["~/.cursor/mcp.json"],
  notes: [],
  servers: CURSOR_NAMES.map((name, index) =>
    server({
      name,
      transport: index % 3 === 0 ? "stdio" : "http",
      target: index % 3 === 0 ? `npx -y @${name}/mcp` : `mcp.${name}.com`,
      approval: index === 4 ? "pending" : "approved",
      sourceLabel: "~/.cursor/mcp.json",
    })
  ),
};

export const DEFAULT_RUNTIMES: readonly RuntimeGroupData[] = [CLAUDE, CODEX, PI, GEMINI];

/** The Claude group with its servers off in CompozyOS sessions (US-009). */
export const CLAUDE_OFF: RuntimeGroupData = {
  ...CLAUDE,
  runtimeMcp: { mode: "off", canDisable: true },
};
