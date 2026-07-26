import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const runtimeRoot = resolve(siteRoot, "content/runtime");

function readDoc(...parts: string[]): string {
  return readFileSync(resolve(runtimeRoot, ...parts), "utf8");
}

function readJSON<T>(...parts: string[]): T {
  return JSON.parse(readDoc(...parts)) as T;
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

describe("authored context runtime docs", () => {
  it("documents SOUL.md authoring boundary, lifecycle, and managed surfaces", () => {
    const soul = readDoc("core/agents/soul.mdx");

    expectIncludesAll(soul, [
      "`SOUL.md`",
      "persona",
      "expected_digest",
      "agh agent soul inspect",
      "agh agent soul validate",
      "agh agent soul write",
      "agh agent soul delete",
      "agh agent soul history",
      "agh agent soul rollback",
      "agh session soul refresh",
      "`agents/soul/get`",
      "`agents/soul/put`",
      "`agents/soul/rollback`",
      "parent_soul_digest",
      "`AGENT.md`",
      "Agent Heartbeat",
    ]);
    expectExcludesAll(soul, ["agh session heartbeat", "$CLAIM_TOKEN"]);
  });

  it("documents HEARTBEAT.md as advisory wake policy with no queue or task ownership", () => {
    const heartbeat = readDoc("core/agents/heartbeat.mdx");

    expectIncludesAll(heartbeat, [
      "`HEARTBEAT.md`",
      "advisory",
      "synthetic wake",
      "session health",
      "wake policy",
      "expected_digest",
      "heartbeat_if_match_header_unsupported",
      "agh agent heartbeat inspect",
      "agh agent heartbeat validate",
      "agh agent heartbeat write",
      "agh agent heartbeat delete",
      "agh agent heartbeat history",
      "agh agent heartbeat rollback",
      "agh agent heartbeat status",
      "agh agent heartbeat wake",
      "`agents/heartbeat/wake`",
      "`agents/heartbeat/status`",
      "session_prompt_active_race",
      "wake_coalesced",
    ]);
    expectExcludesAll(heartbeat, ["agh session heartbeat", "$CLAIM_TOKEN", 'claim_token":']);
    // Heartbeat must explicitly call out the absent refresh command, not advertise it.
    expect(heartbeat).toContain("no `agh agent heartbeat refresh` command");
  });

  it("registers SOUL.md and HEARTBEAT.md pages in agents navigation", () => {
    const meta = readJSON<{ pages: string[] }>("core/agents/meta.json");
    expect(meta.pages).toContain("soul");
    expect(meta.pages).toContain("heartbeat");
    expect(existsSync(resolve(runtimeRoot, "core/agents/soul.mdx"))).toBe(true);
    expect(existsSync(resolve(runtimeRoot, "core/agents/heartbeat.mdx"))).toBe(true);
  });

  it("documents session health as metadata-only with deterministic wake reasons", () => {
    const health = readDoc("core/sessions/health.mdx");
    const sessionsMeta = readJSON<{ pages: string[] }>("core/sessions/meta.json");

    expect(sessionsMeta.pages).toContain("health");
    expectIncludesAll(health, [
      "metadata-only",
      "eligible_for_wake",
      "session_unhealthy",
      "session_not_attachable",
      "session_prompt_active",
      "session_prompt_active_race",
      "cooldown_active",
      "quiet_window",
      "heartbeat_disabled",
      "heartbeat_invalid",
      "heartbeat_no_policy",
      "heartbeat_rate_limited",
      "wake_coalesced",
      "agh session health",
      "agh session status",
      "agh session inspect",
      "include_health=true",
      "AGH Network membership",
    ]);
  });

  it("documents [agents.soul] and [agents.heartbeat] config sections with defaults", () => {
    const config = readDoc("core/configuration/config-toml.mdx");

    expectIncludesAll(config, [
      "`[agents.soul]`",
      "`[agents.heartbeat]`",
      "max_body_bytes",
      "context_projection_bytes",
      "min_interval",
      "wake_cooldown",
      "max_wakes_per_cycle",
      "active_session_only",
      "wake_event_retention",
      "session_health_stale_after",
      "session_health_hook_min_interval",
      "Optional `SOUL.md`",
      "advisory wake policy",
    ]);
  });

  it("documents authored-context Host API grants, hooks, and native tools in extensions", () => {
    const develop = readDoc("core/extensions/develop.mdx");

    expectIncludesAll(develop, [
      "agents/soul/get",
      "agents/soul/validate",
      "agents/soul/put",
      "agents/soul/delete",
      "agents/soul/history",
      "agents/soul/rollback",
      "agents/heartbeat/get",
      "agents/heartbeat/validate",
      "agents/heartbeat/put",
      "agents/heartbeat/delete",
      "agents/heartbeat/history",
      "agents/heartbeat/rollback",
      "agents/heartbeat/status",
      "agents/heartbeat/wake",
      "sessions/health/get",
      "agent.soul.snapshot.resolved",
      "agent.soul.mutation.after",
      "agent.heartbeat.policy.resolved",
      "agent.heartbeat.wake.before",
      "agent.heartbeat.wake.after",
      "session.health.update.after",
      "agh__session_health",
      "agh__agent_heartbeat_status",
      "agh__agent_heartbeat_wake",
    ]);
    // No native tool for Soul exists today.
    expect(develop).not.toContain("agh__agent_soul ");
  });

  it("documents AGH Network greet as independent from authored context", () => {
    const protocol = readDoc("core/network/protocol.mdx");

    expectIncludesAll(protocol, [
      "Network participation is independent from authored context",
      "greet",
      "`SOUL.md`",
      "`HEARTBEAT.md`",
      "wake-eligible",
    ]);
  });

  it("ships generated CLI references for soul, heartbeat, and session health/status/inspect", () => {
    const required = [
      "cli-reference/agent/soul/index.mdx",
      "cli-reference/agent/soul/inspect.mdx",
      "cli-reference/agent/soul/validate.mdx",
      "cli-reference/agent/soul/write.mdx",
      "cli-reference/agent/soul/delete.mdx",
      "cli-reference/agent/soul/history.mdx",
      "cli-reference/agent/soul/rollback.mdx",
      "cli-reference/agent/heartbeat/index.mdx",
      "cli-reference/agent/heartbeat/inspect.mdx",
      "cli-reference/agent/heartbeat/validate.mdx",
      "cli-reference/agent/heartbeat/write.mdx",
      "cli-reference/agent/heartbeat/delete.mdx",
      "cli-reference/agent/heartbeat/history.mdx",
      "cli-reference/agent/heartbeat/rollback.mdx",
      "cli-reference/agent/heartbeat/status.mdx",
      "cli-reference/agent/heartbeat/wake.mdx",
      "cli-reference/session/soul/index.mdx",
      "cli-reference/session/soul/refresh.mdx",
      "cli-reference/session/health.mdx",
      "cli-reference/session/status.mdx",
      "cli-reference/session/inspect.mdx",
    ];
    for (const page of required) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("does not advertise an agh session heartbeat command in CLI references", () => {
    expect(existsSync(resolve(runtimeRoot, "cli-reference/session/heartbeat"))).toBe(false);
    expect(existsSync(resolve(runtimeRoot, "cli-reference/session/heartbeat.mdx"))).toBe(false);

    const sessionIndex = readDoc("cli-reference/session/index.mdx");
    expect(sessionIndex).not.toContain("agh session heartbeat");
  });
});
