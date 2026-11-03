// S9: the wire review → what the card and viewer show. "Runs automatically" follows the daemon's
// own inventory (`internal/agentinstance/skill_automatic.go`): scripts under `scripts/`, servers in
// `mcp.json`, and hooks or MCP servers declared in SKILL.md metadata.
import type { AgentSkillReviewPayload, AgentSkillReviewState } from "../types";
import { parseFrontmatter, splitFrontmatter } from "./agent-frontmatter";

export const SKILL_DEFINITION_FILE = "SKILL.md";
const MCP_JSON_NAME = "mcp.json";
const SCRIPTS_DIR = "scripts/";

export interface SkillReviewFile {
  path: string;
  language: string;
  body: string;
  /** A file that declares or is something that runs on its own. */
  automatic: boolean;
}

export type SkillAutomaticKind = "hook" | "mcp" | "script";

export interface SkillAutomaticItem {
  kind: SkillAutomaticKind;
  label: string;
  /** The files that declare it. */
  detail: string;
}

const LANGUAGE_BY_EXTENSION: Readonly<Record<string, string>> = {
  md: "markdown",
  markdown: "markdown",
  json: "json",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  py: "python",
  js: "javascript",
  mjs: "javascript",
  ts: "typescript",
  yaml: "yaml",
  yml: "yaml",
  toml: "toml",
};

function baseName(path: string) {
  return path.split("/").at(-1) ?? path;
}

export function skillFileLanguage(path: string) {
  const name = baseName(path);
  const dot = name.lastIndexOf(".");
  if (dot <= 0) return "text";
  return LANGUAGE_BY_EXTENSION[name.slice(dot + 1).toLowerCase()] ?? "text";
}

function definitionMetadata(content: string) {
  const split = splitFrontmatter(content);
  return split?.metadata ? parseFrontmatter(split.metadata) : null;
}

function compozyMetadata(content: string) {
  const metadata = definitionMetadata(content)?.metadata;
  const compozy =
    metadata && typeof metadata === "object" && !Array.isArray(metadata)
      ? (metadata as Record<string, unknown>).compozy
      : undefined;
  return compozy && typeof compozy === "object" && !Array.isArray(compozy)
    ? (compozy as Record<string, unknown>)
    : {};
}

function declares(content: string) {
  const compozy = compozyMetadata(content);
  const count = (key: string) => (Array.isArray(compozy[key]) ? compozy[key].length : 0);
  return { hooks: count("hooks"), mcp: count("mcp_servers") };
}

function isAutomatic(path: string, content: string) {
  if (path.startsWith(SCRIPTS_DIR) || baseName(path) === MCP_JSON_NAME) return true;
  if (path !== SKILL_DEFINITION_FILE) return false;
  const found = declares(content);
  return found.hooks + found.mcp > 0;
}

/** Every file, SKILL.md first, then by path, as the daemon stores them. */
export function skillReviewFiles(review: AgentSkillReviewPayload): SkillReviewFile[] {
  return (review.files ?? [])
    .map(file => ({
      path: file.path,
      language: skillFileLanguage(file.path),
      body: file.content,
      automatic: isAutomatic(file.path, file.content),
    }))
    .sort((a, b) =>
      a.path === SKILL_DEFINITION_FILE ? -1 : b.path === SKILL_DEFINITION_FILE ? 1 : 0
    );
}

/** Why the agent proposed it: the description it wrote into SKILL.md. */
export function skillReviewWhy(review: AgentSkillReviewPayload) {
  const definition = review.files?.find(file => file.path === SKILL_DEFINITION_FILE);
  const description = definition ? definitionMetadata(definition.content)?.description : undefined;
  return typeof description === "string" && description.trim() ? description.trim() : undefined;
}

function plural(count: number, one: string, many: string) {
  return `${count} ${count === 1 ? one : many}`;
}

/** Hooks and MCP servers first, then scripts, each naming the files that declare it. */
export function skillAutomaticItems(
  review: AgentSkillReviewPayload,
  files: readonly SkillReviewFile[] = skillReviewFiles(review)
): SkillAutomaticItem[] {
  const counts = review.runs_automatically;
  const where = (match: (file: SkillReviewFile) => boolean) =>
    files.flatMap(file => (file.automatic && match(file) ? [file.path] : [])).join(", ");
  const items: SkillAutomaticItem[] = [];
  if (counts.hooks > 0) {
    items.push({
      kind: "hook",
      label: `${plural(counts.hooks, "hook runs", "hooks run")} when events happen`,
      detail: where(file => file.path === SKILL_DEFINITION_FILE) || SKILL_DEFINITION_FILE,
    });
  }
  if (counts.mcp_servers > 0) {
    items.push({
      kind: "mcp",
      label: `Starts ${plural(counts.mcp_servers, "MCP server", "MCP servers")}`,
      detail:
        where(
          file => baseName(file.path) === MCP_JSON_NAME || file.path === SKILL_DEFINITION_FILE
        ) || SKILL_DEFINITION_FILE,
    });
  }
  if (counts.scripts > 0) {
    items.push({
      kind: "script",
      label: `Includes ${plural(counts.scripts, "script", "scripts")} it can run`,
      detail: where(file => file.path.startsWith(SCRIPTS_DIR)),
    });
  }
  return items;
}

export function skillReviewState(review: AgentSkillReviewPayload): AgentSkillReviewState {
  return review.state === "approved" || review.state === "rejected" ? review.state : "pending";
}
