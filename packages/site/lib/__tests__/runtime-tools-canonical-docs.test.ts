import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const runtimeRoot = resolve(siteRoot, "content/docs");

function readDoc(...parts: string[]): string {
  return readFileSync(resolve(runtimeRoot, ...parts), "utf8");
}

function expectIncludesAll(content: string, values: string[]): void {
  for (const value of values) {
    expect(content).toContain(value);
  }
}

function expectExcludesAll(content: string, values: string[]): void {
  for (const value of values) {
    expect(content).not.toContain(value);
  }
}

describe("tool-first canonical surface docs", () => {
  it("documents default discovery toolsets and the tool/operator split in agent docs", () => {
    const agentMd = readDoc("configuration/agent-md.mdx");
    const definitions = readDoc("agents/definitions.mdx");

    expectIncludesAll(agentMd, [
      "`compozy__bootstrap`",
      "`compozy__catalog`",
      "`compozy__tool_search`",
      "`compozy__tool_info`",
      "Operator-only management",
      "MCP OAuth login/logout",
    ]);
    expectIncludesAll(definitions, [
      "`compozy__bootstrap` and `compozy__catalog`",
      "resolve canonical `compozy__tool_search`",
      "canonical `compozy__tool_info`",
      "harness-returned tool reference",
      "canonical `compozy__skill_search`",
      "and `compozy__skill_view`",
      "Operator-only management",
    ]);
  });

  it("documents operational native toolsets for task, model catalog, and Memory administration", () => {
    const toolsets = readDoc("tools/toolsets.mdx");
    const tools = readDoc("tools/index.mdx");

    expectIncludesAll(toolsets, [
      "compozy__tasks",
      "compozy__provider_models",
      "compozy__memory",
      "compozy__memory_admin",
      "compozy__memory_admin_history",
      "compozy__bundles",
      "compozy__resources",
      "compozy__mcp",
    ]);
    expectExcludesAll(toolsets, ["compozy__memory_history"]);
    expectIncludesAll(tools, [
      "task bridge notification subscription management",
      "Memory v2 operational/admin actions",
      "extension bundle activation and desired-state resource inspection",
      "MCP server probe/status diagnostics",
    ]);
  });

  it("documents tool-callable autonomy without raw claim tokens", () => {
    const leases = readDoc("autonomy/task-runs-and-leases.mdx");
    const autonomyIndex = readDoc("autonomy/index.mdx");

    expectIncludesAll(leases, [
      "compozy__task_run_claim_next",
      "compozy__task_run_heartbeat",
      "compozy__task_run_complete",
      "compozy__task_run_fail",
      "compozy__task_run_release",
      "AUTONOMY_SESSION_REQUIRED",
      "AUTONOMY_NO_ACTIVE_LEASE",
      "AUTONOMY_FOREIGN_RUN",
      "AUTONOMY_LEASE_EXPIRED",
      "AUTONOMY_LEASE_ALREADY_HELD",
    ]);
    expectExcludesAll(leases, ["--claim-token", '"claim_token":']);
    expectIncludesAll(autonomyIndex, [
      "`compozy__autonomy`",
      "compozy__task_run_claim_next",
      "compozy__task_run_heartbeat",
    ]);
  });

  it("documents tool-callable hooks, automation, and extension lifecycle", () => {
    const hooksIndex = readDoc("hooks/index.mdx");
    const hooksDecl = readDoc("hooks/declaration.mdx");
    const automationIndex = readDoc("automation/index.mdx");
    const extensionsInstall = readDoc("extensions/install.mdx");

    expectIncludesAll(hooksIndex, [
      "`compozy__hooks`",
      "`compozy__hooks_create`",
      "`compozy__hooks_update`",
      "`compozy__hooks_delete`",
      "HOOK_SOURCE_IMMUTABLE",
      "HOOK_APPROVAL_REQUIRED",
    ]);
    expectIncludesAll(hooksDecl, [
      "`compozy__hooks_list`",
      "`compozy__hooks_info`",
      "`compozy__hooks_events`",
      "`compozy__hooks_runs`",
      "HOOK_SOURCE_IMMUTABLE",
    ]);
    expectIncludesAll(automationIndex, [
      "`compozy__automation`",
      "`compozy__automation_jobs_create`",
      "`compozy__automation_triggers_create`",
      "AUTOMATION_SCOPE_FORBIDDEN",
      "AUTOMATION_SECRET_INPUT_FORBIDDEN",
      "webhook_secret_ref",
    ]);
    expectIncludesAll(extensionsInstall, [
      "`compozy__extensions`",
      "`compozy__extensions_install`",
      "`compozy__extensions_remove`",
      "`compozy__bundles_activate`",
      "`compozy__resources_snapshot`",
      "`resources/list`",
      "`resources/get`",
      "`resources/snapshot`",
      "EXTENSION_SOURCE_FORBIDDEN",
      "EXTENSION_APPROVAL_REQUIRED",
    ]);
  });

  it("documents MCP auth as status-only on the tool surface", () => {
    const configToml = readDoc("configuration/config-toml.mdx");

    expectIncludesAll(configToml, [
      "`compozy__mcp_status`",
      "`compozy__mcp_auth_status`",
      "operator flows available through CLI, HTTP, and UDS",
      "omitted from callable discovery",
      "compozy mcp auth login",
      "compozy mcp auth logout",
    ]);
  });

  it("documents restart-required lifecycle truth for config-backed hook and extension mutations", () => {
    const configToml = readDoc("configuration/config-toml.mdx");

    expectIncludesAll(configToml, [
      "`hooks.*` and `extensions.*`",
      "`applied=false`",
      '`lifecycle="restart-required"`',
      '`next_action="restart-daemon"`',
    ]);
  });

  it("documents the canonical tool surface in skills guidance", () => {
    const skillsIndex = readDoc("skills/index.mdx");
    const bundled = readDoc("skills/bundled.mdx");

    expectIncludesAll(skillsIndex, [
      "`compozy__skill_view`",
      "`compozy__skill_search`",
      "operator CLI",
    ]);
    expectIncludesAll(bundled, [
      "`compozy`",
      "references/native-tools.md",
      "Compozy-native tools",
      "compozy__tool_search",
    ]);
  });

  it("ships generated CLI references for the tool and toolsets command groups", () => {
    const required = [
      "cli/tool/index.mdx",
      "cli/tool/list.mdx",
      "cli/tool/search.mdx",
      "cli/tool/info.mdx",
      "cli/tool/invoke.mdx",
      "cli/toolsets/index.mdx",
      "cli/toolsets/list.mdx",
      "cli/toolsets/info.mdx",
    ];
    for (const page of required) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("does not advertise raw claim_token CLI flags in autonomy CLI pages", () => {
    const cliPages = [
      "cli/task/heartbeat.mdx",
      "cli/task/complete.mdx",
      "cli/task/fail.mdx",
      "cli/task/release.mdx",
      "cli/task/next.mdx",
    ];
    for (const page of cliPages) {
      const content = readDoc(page);
      expectExcludesAll(content, ["--claim-token", "$CLAIM_TOKEN"]);
    }
  });
});
