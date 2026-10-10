// One line per tool call: a verb in the sans ink and the object it acted on
// ("Read  session-thread.tsx", "Ran  go test ./...", "Searched  TODO"). The
// kind comes from the registered tool name first, then — for tools a provider
// names by title ("Terminal") or that no catalog knows — from the input shape,
// and simple read/list/search shell commands read as that action. The raw
// name, title, and input stay one click deeper (body, copy, find).

import {
  Bot,
  FileEdit,
  FileText,
  FolderOpen,
  Globe,
  Search,
  Sparkles,
  Terminal,
  type LucideIcon,
} from "lucide-react";

import { compactSessionSummary } from "./session-summary";
import type { SessionToolKind } from "./session-tool-visual-state";
import { inferShellIntent, stripLeadingCd } from "./tool-shell-intent";
import {
  getToolCompactSummary,
  getToolIcon,
  getToolLabel,
  humanizeToolId,
  isRegisteredToolName,
  resolveRegisteredToolName,
  toolHeadingName,
} from "./tool-labels";

export type ToolDisplayTense = "active" | "past";

/** How the row renders its object: a mono chip (`file`, `code`) or plain sans text. */
export type ToolDisplayTargetKind = "file" | "code" | "text";

export interface ToolDisplay {
  kind: SessionToolKind;
  /** The action in the requested tense ("Read", "Running"). */
  verb: string;
  /** The object of the verb, display-compacted; `fullTarget` keeps the original. */
  target?: string;
  fullTarget?: string;
  targetKind?: ToolDisplayTargetKind;
  icon: LucideIcon;
}

export interface ToolDisplayInput {
  toolName: string;
  toolTitle?: string;
  args?: Record<string, unknown>;
}

type Verbs = Record<ToolDisplayTense, string>;

const VERBS = {
  run: { active: "Running", past: "Ran" },
  read: { active: "Reading", past: "Read" },
  list: { active: "Listing", past: "Listed" },
  search: { active: "Searching", past: "Searched" },
  find: { active: "Finding", past: "Found" },
  webSearch: { active: "Searching the web", past: "Searched the web" },
  fetch: { active: "Fetching", past: "Fetched" },
  edit: { active: "Editing", past: "Edited" },
  write: { active: "Writing", past: "Wrote" },
  agent: { active: "Running agent", past: "Ran agent" },
  skill: { active: "Loading skill", past: "Loaded skill" },
  use: { active: "Running", past: "Used" },
  useTool: { active: "Running tool", past: "Used tool" },
} satisfies Record<string, Verbs>;

const TARGET_LIMIT = 120;
/** A file object keeps its last segments; the full path stays in `fullTarget`. */
const PATH_TAIL_SEGMENTS = 3;

function compactPath(path: string): string {
  const segments = path.split("/").filter(segment => segment.length > 0);
  if (segments.length <= PATH_TAIL_SEGMENTS) return path;
  return `…/${segments.slice(-PATH_TAIL_SEGMENTS).join("/")}`;
}

function stringArg(args: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const key of keys) {
    const value = args[key];
    if (typeof value === "string" && value.trim().length > 0) return value.trim();
  }
  return undefined;
}

function firstLine(text: string): string {
  return text.split(/\r?\n/u).find(line => line.trim().length > 0) ?? text;
}

function display(
  kind: SessionToolKind,
  verbs: Verbs,
  tense: ToolDisplayTense,
  icon: LucideIcon,
  target?: string,
  targetKind: ToolDisplayTargetKind = "code"
): ToolDisplay {
  if (!target) return { kind, verb: verbs[tense], icon };
  const shown = targetKind === "file" ? compactPath(target) : target;
  return {
    kind,
    verb: verbs[tense],
    target: compactSessionSummary(shown, TARGET_LIMIT),
    fullTarget: target,
    targetKind,
    icon,
  };
}

function commandDisplay(command: string, tense: ToolDisplayTense): ToolDisplay {
  const intent = inferShellIntent(command);
  switch (intent?.action) {
    case "read":
      return display("read", VERBS.read, tense, FileText, intent.target, "file");
    case "list":
      return display("read", VERBS.list, tense, FolderOpen, intent.target, "file");
    case "search":
      return display("search", VERBS.search, tense, Search, intent.target);
    default:
      return display("command", VERBS.run, tense, Terminal, firstLine(stripLeadingCd(command)));
  }
}

function filePath(args: Record<string, unknown>): string | undefined {
  return stringArg(args, "file_path", "filePath", "notebook_path");
}

/** The kind a catalogued tool name carries, or `null` when the name says nothing. */
function registeredDisplay(
  name: string,
  args: Record<string, unknown>,
  tense: ToolDisplayTense
): ToolDisplay | null {
  switch (name) {
    case "Bash": {
      const command = stringArg(args, "command");
      return command
        ? commandDisplay(command, tense)
        : display("command", VERBS.run, tense, Terminal, undefined);
    }
    case "Read":
      return display("read", VERBS.read, tense, FileText, filePath(args), "file");
    case "Write":
      return display("edit", VERBS.write, tense, FileEdit, filePath(args), "file");
    case "Edit":
    case "NotebookEdit":
      return display("edit", VERBS.edit, tense, FileEdit, filePath(args), "file");
    case "Grep":
      return display("search", VERBS.search, tense, Search, stringArg(args, "pattern"));
    case "Glob":
      return display("search", VERBS.find, tense, Search, stringArg(args, "pattern"));
    case "WebSearch":
      return display("web", VERBS.webSearch, tense, Globe, stringArg(args, "query"), "text");
    case "WebFetch":
      return display("web", VERBS.fetch, tense, Globe, stringArg(args, "url"));
    case "Task":
    case "Agent":
      return display(
        "agent",
        VERBS.agent,
        tense,
        Bot,
        stringArg(args, "description", "prompt"),
        "text"
      );
    case "Skill":
      return display("other", VERBS.skill, tense, Sparkles, stringArg(args, "skill", "name"));
    default:
      return null;
  }
}

/** Input-shape inference for tools no catalog names (title-named ACP tools, MCP, dynamic). */
function inferredDisplay(
  name: string,
  args: Record<string, unknown>,
  tense: ToolDisplayTense
): ToolDisplay | null {
  const command = stringArg(args, "command", "cmd");
  if (command) return commandDisplay(command, tense);
  const path = filePath(args);
  if (path && ("old_string" in args || "new_string" in args || "edits" in args)) {
    return display("edit", VERBS.edit, tense, FileEdit, path, "file");
  }
  if (path && "content" in args) {
    return display("edit", VERBS.write, tense, FileEdit, path, "file");
  }
  if (path) return display("read", VERBS.read, tense, FileText, path, "file");
  const pattern = stringArg(args, "pattern");
  if (pattern) return display("search", VERBS.search, tense, Search, pattern);
  const url = stringArg(args, "url");
  if (url && /fetch|browse|open|url/iu.test(name)) {
    return display("web", VERBS.fetch, tense, Globe, url);
  }
  return null;
}

/** Catalogued labels ("Updated tasks", "Opened terminal") keep their own verb phrase. */
function labelledDisplay(name: string, args: Record<string, unknown>, tense: ToolDisplayTense) {
  const verb = getToolLabel(name, tense).replace(/…$/u, "");
  const summary = getToolCompactSummary(name, args);
  return {
    kind: "other" as const,
    verb,
    icon: getToolIcon(name, args),
    ...(summary ? { target: summary, fullTarget: summary, targetKind: "text" as const } : {}),
  };
}

/** Resolves the row's verb, object, kind, and glyph for one tool call. */
export function resolveToolDisplay(
  { toolName, toolTitle, args = {} }: ToolDisplayInput,
  tense: ToolDisplayTense
): ToolDisplay {
  const name = resolveRegisteredToolName(toolName);
  const registered = registeredDisplay(name, args, tense);
  if (registered) return registered;
  if (isRegisteredToolName(name)) return labelledDisplay(name, args, tense);
  const inferred = inferredDisplay(name, args, tense);
  if (inferred) return inferred;

  // A free-form provider title ("Run the migration") is the object, never the
  // verb: the verb slot never truncates, so prose stays where it can.
  const heading = toolHeadingName(name);
  const prose = heading === "tool" ? name : toolTitle && toolTitle !== name ? toolTitle : null;
  if (prose && prose !== "tool") {
    return display(
      "other",
      VERBS.useTool,
      tense,
      getToolIcon(name, args),
      firstLine(prose),
      "text"
    );
  }
  return display("other", VERBS.use, tense, getToolIcon(name, args), humanizeToolId(name), "text");
}

/** The full one-line sentence for titles, live status, and accessible names. */
export function toolDisplaySentence(value: ToolDisplay): string {
  return value.target ? `${value.verb} ${value.target}` : value.verb;
}
