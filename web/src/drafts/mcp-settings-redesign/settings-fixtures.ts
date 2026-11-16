/**
 * Draft-local view model for the proposed MCP surfaces. One status vocabulary for every surface
 * (composer, Settings, Marketplace, agents): Ready · Sign in · Review · Not working · Off. Each
 * maps from today's wire states (`mcp-status-view-model.ts`, `marketplace-server-status.ts`,
 * `session-mcp-view.ts`); the mapping is in this draft's README notes in `index.proto.tsx`.
 */

export type ServerStatus = "ready" | "sign-in" | "review" | "not-working" | "off";

export type ServerSource =
  | { kind: "you" }
  | { kind: "project-file" }
  | { kind: "extension"; name: string }
  | { kind: "copied"; from: string };

export interface ServerModel {
  name: string;
  /** Remote servers have a URL; local ones run a command on this computer. */
  transport: "remote" | "local";
  endpoint: string;
  source: ServerSource;
  status: ServerStatus;
  scope: "personal" | "project";
  tools: string[];
  signedInAs?: string;
  /** Names of secret values the server needs (never the values). */
  secrets?: string[];
  reason?: string;
}

const tools = (prefix: string, names: string[]) => names.map(name => `${prefix}${name}`);

export const SERVERS: ServerModel[] = [
  {
    name: "branas-dashboard",
    transport: "remote",
    endpoint: "https://dashboard.branas.dev/mcp",
    source: { kind: "you" },
    status: "sign-in",
    scope: "personal",
    tools: [],
  },
  {
    name: "postgres-local",
    transport: "local",
    endpoint: "npx -y @modelcontextprotocol/server-postgres postgres://localhost:5432/app",
    source: { kind: "project-file" },
    status: "review",
    scope: "project",
    tools: [],
  },
  {
    name: "linear",
    transport: "remote",
    endpoint: "https://mcp.linear.app/mcp",
    source: { kind: "extension", name: "Linear" },
    status: "ready",
    scope: "personal",
    signedInAs: "pedro@compozy.com",
    tools: tools("", [
      "Create issue",
      "Update issue",
      "Search issues",
      "List projects",
      "Get issue",
      "Add comment",
      "List teams",
      "List cycles",
      "Get document",
      "List labels",
    ]),
  },
  {
    name: "github",
    transport: "local",
    endpoint: "github-mcp-server stdio",
    source: { kind: "you" },
    status: "ready",
    scope: "personal",
    secrets: ["GITHUB_TOKEN"],
    tools: tools("", [
      "Create pull request",
      "Search code",
      "Get file",
      "List issues",
      "Merge pull request",
    ]),
  },
  {
    name: "sentry",
    transport: "remote",
    endpoint: "https://mcp.sentry.dev/mcp",
    source: { kind: "copied", from: "Claude Code" },
    status: "ready",
    scope: "personal",
    signedInAs: "pedro@compozy.com",
    tools: tools("", ["Search issues", "Get issue", "Resolve issue"]),
  },
  {
    name: "pagerduty",
    transport: "local",
    endpoint: "npx -y pagerduty-mcp",
    source: { kind: "you" },
    status: "not-working",
    scope: "personal",
    secrets: ["PAGERDUTY_API_KEY"],
    reason: "Server exited with code 1: PAGERDUTY_API_KEY is not set",
    tools: [],
  },
];

export interface AgentServerModel {
  name: string;
  transport: "remote" | "local";
  /** Already copied into CompozyOS. */
  inCompozy?: boolean;
  off?: boolean;
  waiting?: boolean;
  secrets?: string[];
}

export interface CodingAgentModel {
  id: string;
  name: string;
  /** Sessions load this agent's own servers. */
  loads: boolean;
  servers: AgentServerModel[];
}

export const CODING_AGENTS: CodingAgentModel[] = [
  {
    id: "claude",
    name: "Claude Code",
    loads: true,
    servers: [
      { name: "DaVinci Resolve", transport: "local" },
      { name: "Neon", transport: "remote" },
      { name: "context7", transport: "remote", secrets: ["CONTEXT7_API_KEY"] },
      { name: "sentry", transport: "remote", inCompozy: true },
      { name: "cua-driver", transport: "local" },
      { name: "higgsfield", transport: "remote" },
      { name: "inspo", transport: "remote" },
      { name: "figma", transport: "remote", waiting: true },
      { name: "vercel", transport: "remote", off: true },
    ],
  },
  {
    id: "codex",
    name: "Codex",
    loads: false,
    servers: [
      { name: "github", transport: "local", inCompozy: true },
      { name: "local-scripts", transport: "local" },
    ],
  },
];

export function sourceLabel(server: ServerModel): string {
  const where = server.transport === "remote" ? "Remote" : "Runs on this computer";
  switch (server.source.kind) {
    case "you":
      return `${where} · Added by you`;
    case "project-file":
      return `${where} · Added by a file in this project`;
    case "extension":
      return `From the ${server.source.name} extension`;
    case "copied":
      return `${where} · Copied from ${server.source.from}`;
  }
}

export function serverByName(name: string): ServerModel {
  const found = SERVERS.find(server => server.name === name);
  if (!found) throw new Error(`unknown server ${name}`);
  return found;
}
