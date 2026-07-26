import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const runtimeRoot = resolve(siteRoot, "content/runtime");

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
    const agentMd = readDoc("core/configuration/agent-md.mdx");
    const definitions = readDoc("core/agents/definitions.mdx");

    expectIncludesAll(agentMd, [
      "`agh__bootstrap`",
      "`agh__catalog`",
      "`agh__tool_search`",
      "`agh__tool_info`",
      "Operator-only management",
      "MCP OAuth login/logout",
    ]);
    expectIncludesAll(definitions, [
      "`agh__bootstrap` and `agh__catalog`",
      "resolve canonical `agh__tool_search`",
      "canonical `agh__tool_info`",
      "harness-returned tool reference",
      "canonical `agh__skill_search`",
      "and `agh__skill_view`",
      "Operator-only management",
    ]);
  });

  it("documents operational native toolsets for task, model catalog, and Memory administration", () => {
    const toolsets = readDoc("core/tools/toolsets.mdx");
    const tools = readDoc("core/tools/index.mdx");

    expectIncludesAll(toolsets, [
      "agh__tasks",
      "agh__provider_models",
      "agh__memory",
      "agh__memory_admin",
      "agh__memory_admin_history",
      "agh__bundles",
      "agh__resources",
      "agh__mcp",
    ]);
    expectExcludesAll(toolsets, ["agh__memory_history"]);
    expectIncludesAll(tools, [
      "task bridge notification subscription management",
      "Memory v2 operational/admin actions",
      "extension bundle activation and desired-state resource inspection",
      "MCP server probe/status diagnostics",
    ]);
  });

  it("documents tool-callable autonomy without raw claim tokens", () => {
    const leases = readDoc("core/autonomy/task-runs-and-leases.mdx");
    const autonomyIndex = readDoc("core/autonomy/index.mdx");

    expectIncludesAll(leases, [
      "agh__task_run_claim_next",
      "agh__task_run_heartbeat",
      "agh__task_run_complete",
      "agh__task_run_fail",
      "agh__task_run_release",
      "AUTONOMY_SESSION_REQUIRED",
      "AUTONOMY_NO_ACTIVE_LEASE",
      "AUTONOMY_FOREIGN_RUN",
      "AUTONOMY_LEASE_EXPIRED",
      "AUTONOMY_LEASE_ALREADY_HELD",
    ]);
    expectExcludesAll(leases, ["--claim-token", '"claim_token":']);
    expectIncludesAll(autonomyIndex, [
      "`agh__autonomy`",
      "agh__task_run_claim_next",
      "agh__task_run_heartbeat",
    ]);
  });

  it("documents tool-callable hooks, automation, and extension lifecycle", () => {
    const hooksIndex = readDoc("core/hooks/index.mdx");
    const hooksDecl = readDoc("core/hooks/declaration.mdx");
    const automationIndex = readDoc("core/automation/index.mdx");
    const extensionsInstall = readDoc("core/extensions/install.mdx");

    expectIncludesAll(hooksIndex, [
      "`agh__hooks`",
      "`agh__hooks_create`",
      "`agh__hooks_update`",
      "`agh__hooks_delete`",
      "HOOK_SOURCE_IMMUTABLE",
      "HOOK_APPROVAL_REQUIRED",
    ]);
    expectIncludesAll(hooksDecl, [
      "`agh__hooks_list`",
      "`agh__hooks_info`",
      "`agh__hooks_events`",
      "`agh__hooks_runs`",
      "HOOK_SOURCE_IMMUTABLE",
    ]);
    expectIncludesAll(automationIndex, [
      "`agh__automation`",
      "`agh__automation_jobs_create`",
      "`agh__automation_triggers_create`",
      "AUTOMATION_SCOPE_FORBIDDEN",
      "AUTOMATION_SECRET_INPUT_FORBIDDEN",
      "webhook_secret_ref",
    ]);
    expectIncludesAll(extensionsInstall, [
      "`agh__extensions`",
      "`agh__extensions_install`",
      "`agh__extensions_remove`",
      "`agh__bundles_activate`",
      "`agh__resources_snapshot`",
      "`resource.write` capability names are not accepted",
      "EXTENSION_SOURCE_FORBIDDEN",
      "EXTENSION_APPROVAL_REQUIRED",
    ]);
  });

  it("documents MCP auth as status-only on the tool surface", () => {
    const configToml = readDoc("core/configuration/config-toml.mdx");

    expectIncludesAll(configToml, [
      "`agh__mcp_status`",
      "`agh__mcp_auth_status`",
      "operator flows available through CLI, HTTP, and UDS",
      "omitted from callable discovery",
      "agh mcp auth login",
      "agh mcp auth logout",
    ]);
  });

  it("documents restart-required lifecycle truth for config-backed hook and extension mutations", () => {
    const configToml = readDoc("core/configuration/config-toml.mdx");

    expectIncludesAll(configToml, [
      "`hooks.*` and `extensions.*`",
      "`applied=false`",
      '`lifecycle="restart-required"`',
      '`next_action="restart-daemon"`',
    ]);
  });

  it("documents the canonical tool surface in skills guidance", () => {
    const skillsIndex = readDoc("core/skills/index.mdx");
    const bundled = readDoc("core/skills/bundled.mdx");

    expectIncludesAll(skillsIndex, ["`agh__skill_view`", "`agh__skill_search`", "operator CLI"]);
    expectIncludesAll(bundled, [
      "`agh`",
      "references/native-tools.md",
      "AGH-native tools",
      "agh__tool_search",
    ]);
  });

  it("ships generated CLI references for the tool and toolsets command groups", () => {
    const required = [
      "cli-reference/tool/index.mdx",
      "cli-reference/tool/list.mdx",
      "cli-reference/tool/search.mdx",
      "cli-reference/tool/info.mdx",
      "cli-reference/tool/invoke.mdx",
      "cli-reference/toolsets/index.mdx",
      "cli-reference/toolsets/list.mdx",
      "cli-reference/toolsets/info.mdx",
    ];
    for (const page of required) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("does not advertise raw claim_token CLI flags in autonomy CLI pages", () => {
    const cliPages = [
      "cli-reference/task/heartbeat.mdx",
      "cli-reference/task/complete.mdx",
      "cli-reference/task/fail.mdx",
      "cli-reference/task/release.mdx",
      "cli-reference/task/next.mdx",
    ];
    for (const page of cliPages) {
      const content = readDoc(page);
      expectExcludesAll(content, ["--claim-token", "$CLAIM_TOKEN"]);
    }
  });
});
