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

/** A YAML key the frontmatter reader takes as is; anything else goes out as JSON. */
const PLAIN_KEY = /^[A-Za-z_][\w-]*$/;

/** JSON is valid YAML flow, so nested values stay exact without a YAML library. */
function yamlValue(value: unknown): string {
  return JSON.stringify(value ?? null);
}

function appendHeartbeatSources(
  lines: string[],
  sources: NonNullable<AgentHeartbeatPayload["frontmatter"]["context"]["sources"]>
): void {
  if (sources.length === 0) return;
  lines.push("  sources:");
  for (const source of sources) {
    lines.push(`    - id: ${quoteYaml(source.id)}`);
    if (source.kind) lines.push(`      kind: ${quoteYaml(source.kind)}`);
    if (source.script) lines.push(`      script: ${quoteYaml(source.script)}`);
    const values = source.with;
    if (values === undefined || values === null) continue;
    const entries =
      typeof values === "object" && !Array.isArray(values)
        ? Object.entries(values as Record<string, unknown>)
        : null;
    if (!entries || entries.length === 0 || entries.some(([key]) => !PLAIN_KEY.test(key))) {
      lines.push(`      with: ${yamlValue(values)}`);
      continue;
    }
    lines.push("      with:");
    for (const [key, value] of entries) lines.push(`        ${key}: ${yamlValue(value)}`);
  }
}

function finishSource(lines: string[], body: string): string {
  const head = `${lines.join("\n")}\n---\n`;
  const normalizedBody = body.endsWith("\n") ? body : `${body}\n`;
  return `${head}${normalizedBody}`;
}

/** Rebuild the managed SOUL.md source from its lossless public read projection. */
export function serializeAgentSoulSource(
  payload: Pick<AgentSoulPayload, "frontmatter" | "body">
): string {
  const frontmatter = payload.frontmatter;
  const lines = ["---"];
  if (frontmatter.version) lines.push(`version: ${quoteYaml(frontmatter.version)}`);
  if (frontmatter.role) lines.push(`role: ${quoteYaml(frontmatter.role)}`);
  appendStringList(lines, "tone", frontmatter.tone ?? []);
  appendStringList(lines, "principles", frontmatter.principles ?? []);
  appendStringList(lines, "constraints", frontmatter.constraints ?? []);
  appendStringList(lines, "collaboration", frontmatter.collaboration ?? []);
  appendStringList(lines, "tags", frontmatter.tags ?? []);
  return finishSource(lines, payload.body ?? "");
}

/** Rebuild the managed HEARTBEAT.md source from its allowlisted read projection. */
export function serializeAgentHeartbeatSource(
  payload: Pick<AgentHeartbeatPayload, "frontmatter" | "guidance_markdown">
): string {
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
  const contextLines: string[] = [];
  // An empty list is a choice: every source off. Only a missing key means all of them.
  if (context.include?.length === 0) contextLines.push("  include: []");
  if (context.include?.length) {
    contextLines.push("  include:");
    for (const value of context.include) {
      contextLines.push(`    - ${quoteYaml(value)}`);
    }
  }
  appendHeartbeatSources(contextLines, context.sources ?? []);
  if (contextLines.length > 0) lines.push("context:", ...contextLines);

  return finishSource(lines, payload.guidance_markdown ?? "");
}
