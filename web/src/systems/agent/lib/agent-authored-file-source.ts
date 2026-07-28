import type { AgentHeartbeatPayload, AgentSoulPayload } from "../types";

function quoteYaml(value: string): string {
  return JSON.stringify(value);
}

function appendStringList(lines: string[], key: string, values: readonly string[]): void {
  if (values.length === 0) return;
  lines.push(`${key}:`);
  for (const value of values) {
    lines.push(`  - ${quoteYaml(value)}`);
  }
}

function appendHeartbeatWindows(
  lines: string[],
  key: string,
  windows: readonly { timezone: string; start: string; end: string }[]
): void {
  if (windows.length === 0) return;
  lines.push(`  ${key}:`);
  for (const window of windows) {
    lines.push(`    - timezone: ${quoteYaml(window.timezone)}`);
    lines.push(`      start: ${quoteYaml(window.start)}`);
    lines.push(`      end: ${quoteYaml(window.end)}`);
  }
}

function finishSource(lines: string[], body: string): string {
  const head = `${lines.join("\n")}\n---\n`;
  const normalizedBody = body.endsWith("\n") ? body : `${body}\n`;
  return `${head}${normalizedBody}`;
}

/** Rebuild the managed SOUL.md source from its lossless public read projection. */
export function serializeAgentSoulSource(payload: AgentSoulPayload): string {
  const frontmatter = payload.frontmatter;
  const lines = ["---"];
  if (frontmatter.version) lines.push(`version: ${quoteYaml(frontmatter.version)}`);
  if (frontmatter.role) lines.push(`role: ${quoteYaml(frontmatter.role)}`);
  appendStringList(lines, "tone", frontmatter.tone ?? []);
  appendStringList(lines, "principles", frontmatter.principles ?? []);
  appendStringList(lines, "constraints", frontmatter.constraints ?? []);
  appendStringList(lines, "collaboration", frontmatter.collaboration ?? []);
  appendStringList(lines, "memory_policy", frontmatter.memory_policy ?? []);
  appendStringList(lines, "tags", frontmatter.tags ?? []);
  return finishSource(lines, payload.body ?? "");
}

/** Rebuild the managed HEARTBEAT.md source from its allowlisted read projection. */
export function serializeAgentHeartbeatSource(payload: AgentHeartbeatPayload): string {
  const frontmatter = payload.frontmatter;
  const preferences = frontmatter.preferences;
  const context = frontmatter.context;
  const activeHours = preferences.active_hours ?? [];
  const quietWindows = preferences.quiet_windows ?? [];
  const lines = ["---", `version: ${frontmatter.version}`, `enabled: ${frontmatter.enabled}`];

  if (frontmatter.summary) lines.push(`summary: ${quoteYaml(frontmatter.summary)}`);
  if (preferences.min_interval || activeHours.length > 0 || quietWindows.length > 0) {
    lines.push("preferences:");
    if (preferences.min_interval) {
      lines.push(`  min_interval: ${quoteYaml(preferences.min_interval)}`);
    }
    appendHeartbeatWindows(lines, "active_hours", activeHours);
    appendHeartbeatWindows(lines, "quiet_windows", quietWindows);
  }
  if ((context.include ?? []).length > 0) {
    lines.push("context:", "  include:");
    for (const value of context.include ?? []) {
      lines.push(`    - ${quoteYaml(value)}`);
    }
  }

  return finishSource(lines, payload.guidance_markdown ?? "");
}
