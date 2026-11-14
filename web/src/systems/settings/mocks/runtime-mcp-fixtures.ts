/**
 * Runtime MCP inventory fixtures in the wire shape of `GET /api/settings/runtime-mcp-servers`
 * (spec `mcp-extension-hot-activation`, `_dx.md`). Names and key names only: the inventory never
 * carries values. Shared by route tests, component tests and the proto draft.
 */
import type { RuntimeMCPInventory, RuntimeMCPServer, RuntimeMCPSnapshot } from "../types";

export function runtimeMCPServerFixture(
  partial: Partial<RuntimeMCPServer> & Pick<RuntimeMCPServer, "name" | "runtime">
): RuntimeMCPServer {
  return {
    scope: "user",
    transport: "stdio",
    source_path: "/Users/me/.claude.json",
    source_label: "~/.claude.json",
    enabled_in_runtime: true,
    approval: "not_required",
    auth: "unknown",
    env_keys: [],
    header_keys: [],
    also_in_compozy: false,
    copyable: true,
    copy_blocked_reason: "",
    definition_digest: `sha256:${partial.runtime}-${partial.name}`,
    ...partial,
  };
}

const claude = (partial: Partial<RuntimeMCPServer> & Pick<RuntimeMCPServer, "name">) =>
  runtimeMCPServerFixture({ runtime: "claude", ...partial });
const codex = (partial: Partial<RuntimeMCPServer> & Pick<RuntimeMCPServer, "name">) =>
  runtimeMCPServerFixture({
    runtime: "codex",
    source_path: "/Users/me/.codex/config.toml",
    source_label: "~/.codex/config.toml",
    ...partial,
  });

export const claudeRuntimeMCPFixture: RuntimeMCPSnapshot = {
  runtime: "claude",
  display_name: "Claude Code",
  installed: true,
  runtime_mcp: { mode: "load", can_disable: true },
  read_state: "ok",
  config_files: ["~/.claude.json", "~/Dev/platform/.mcp.json"],
  notes: ["claude.ai connectors aren't listed here. They show up inside sessions."],
  servers: [
    claude({ name: "context7" }),
    claude({ name: "playwright" }),
    claude({ name: "linear", transport: "http", also_in_compozy: true }),
    claude({ name: "exa", transport: "http", header_keys: ["Authorization"] }),
    claude({
      name: "postgres",
      scope: "local",
      env_keys: ["DATABASE_URL"],
      source_label: "~/.claude.json · this project",
    }),
    claude({
      name: "sentry",
      scope: "project",
      transport: "http",
      approval: "pending",
      source_path: "/Users/me/Dev/platform/.mcp.json",
      source_label: "~/Dev/platform/.mcp.json",
    }),
    claude({ name: "notion", transport: "http", enabled_in_runtime: false }),
    claude({
      name: "plugin:figma:figma",
      scope: "plugin",
      transport: "http",
      source_label: "figma plugin",
      copyable: false,
      copy_blocked_reason: "Plugin servers stay in Claude Code",
    }),
  ],
};

export const claudeOffRuntimeMCPFixture: RuntimeMCPSnapshot = {
  ...claudeRuntimeMCPFixture,
  runtime_mcp: { mode: "off", can_disable: true },
};

export const codexRuntimeMCPFixture: RuntimeMCPSnapshot = {
  runtime: "codex",
  display_name: "Codex",
  installed: true,
  runtime_mcp: { mode: "load", can_disable: false },
  read_state: "ok",
  config_files: ["~/.codex/config.toml"],
  notes: [],
  servers: [
    codex({ name: "github", env_keys: ["GITHUB_PERSONAL_ACCESS_TOKEN"], auth: "not_required" }),
    codex({ name: "linear", transport: "http", also_in_compozy: true }),
    codex({
      name: "local-scripts",
      copyable: false,
      copy_blocked_reason: "Runs in its own folder",
    }),
    codex({ name: "computer-use", enabled_in_runtime: false, auth: "not_required" }),
  ],
};

export const piRuntimeMCPFixture: RuntimeMCPSnapshot = {
  runtime: "pi",
  display_name: "Pi",
  installed: true,
  runtime_mcp: { mode: "load", can_disable: false },
  read_state: "ok",
  config_files: ["~/.pi/agent/mcp.json"],
  notes: [],
  servers: [
    runtimeMCPServerFixture({
      runtime: "pi",
      name: "playwright",
      source_path: "/Users/me/.pi/agent/mcp.json",
      source_label: "~/.pi/agent/mcp.json",
    }),
  ],
};

export const geminiNotInstalledRuntimeMCPFixture: RuntimeMCPSnapshot = {
  runtime: "gemini",
  display_name: "Gemini CLI",
  installed: false,
  runtime_mcp: { mode: "load", can_disable: false },
  read_state: "ok",
  config_files: ["~/.gemini/settings.json"],
  notes: [],
  servers: ["firebase", "exa"].map(name =>
    runtimeMCPServerFixture({
      runtime: "gemini",
      name,
      transport: name === "exa" ? "http" : "stdio",
      header_keys: name === "exa" ? ["x-api-key"] : [],
      source_path: "/Users/me/.gemini/settings.json",
      source_label: "~/.gemini/settings.json",
    })
  ),
};

export const opencodeUnreadableRuntimeMCPFixture: RuntimeMCPSnapshot = {
  runtime: "opencode",
  display_name: "OpenCode",
  installed: true,
  runtime_mcp: { mode: "load", can_disable: false },
  read_state: "unreadable",
  read_error: "Unexpected token at line 14",
  config_files: ["~/.config/opencode/opencode.jsonc"],
  notes: [],
  servers: [],
};

/** Builds a runtime group with `count` servers (folding, 100-server scale). */
export function manyRuntimeMCPFixture(count: number): RuntimeMCPSnapshot {
  return {
    runtime: "cursor",
    display_name: "Cursor Agent",
    installed: true,
    runtime_mcp: { mode: "load", can_disable: false },
    read_state: "ok",
    config_files: ["~/.cursor/mcp.json"],
    notes: [],
    servers: Array.from({ length: count }, (_, index) =>
      runtimeMCPServerFixture({
        runtime: "cursor",
        name: `server-${String(index + 1).padStart(2, "0")}`,
        transport: index % 3 === 0 ? "stdio" : "http",
        source_path: "/Users/me/.cursor/mcp.json",
        source_label: "~/.cursor/mcp.json",
      })
    ),
  };
}

export function runtimeMCPInventoryFixture(
  runtimes: readonly RuntimeMCPSnapshot[] = [
    claudeRuntimeMCPFixture,
    codexRuntimeMCPFixture,
    piRuntimeMCPFixture,
    geminiNotInstalledRuntimeMCPFixture,
  ]
): RuntimeMCPInventory {
  return { runtimes: [...runtimes], generated_at: "2026-11-14T16:30:00Z" };
}
