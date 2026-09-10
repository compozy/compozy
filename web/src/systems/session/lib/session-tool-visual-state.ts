// Tool visual hierarchy (ADR-009): a tool row communicates exactly two axes —
// status (live / settled / absorbed failure / stopped / turn-ending failure /
// empty) and kind (command / edit / read / search / web / agent / other).
// Status is motion, ink, or a trailing glyph; kind is the left glyph from the
// production icon map. Danger is reserved for a failure that ended the turn;
// a failure the agent absorbed reads as information in the settled ink. This
// module owns the mapping and the accessible words; palette fidelity is owned
// by the artboards and Storybook capture, never by a unit test.

import { compactSessionSummary } from "./session-summary";

import { getToolCompactSummary, resolveRegisteredToolName, toolHeadingName } from "./tool-labels";

export type SessionToolVisualStatus =
  | "live"
  | "settled"
  | "absorbed"
  | "stopped"
  | "failed"
  | "empty";

export type SessionToolKind = "command" | "edit" | "read" | "search" | "web" | "agent" | "other";

export interface SessionToolVisualState {
  status: SessionToolVisualStatus;
  kind: SessionToolKind;
  /** Accessible status word; motion and glyphs never carry state alone. */
  statusLabel: string;
}

export interface SessionToolVisualInput {
  /** The transcript lifecycle of the call. */
  status: "running" | "settled" | "interrupted";
  /** The runtime reported the call itself as an error. */
  isError?: boolean;
  /** The settled result carries an error channel (stderr/error/failure-shaped output). */
  resultFailed?: boolean;
  /** The settled result carries no output at all. */
  resultEmpty?: boolean;
  /** The owning turn ended in a turn-level failure; only then a failed call earns danger. */
  turnFailed?: boolean;
}

const STATUS_LABEL: Record<SessionToolVisualStatus, string> = {
  live: "Running",
  settled: "Done",
  absorbed: "Failed, turn continued",
  stopped: "Stopped",
  failed: "Failed",
  empty: "Empty",
};

const COMMAND_TOOLS = new Set(["Bash"]);
const EDIT_TOOLS = new Set(["Edit", "Write", "NotebookEdit"]);
const READ_TOOLS = new Set(["Read"]);
const SEARCH_TOOLS = new Set(["Grep", "Glob"]);
const WEB_TOOLS = new Set(["WebFetch", "WebSearch"]);
const AGENT_TOOLS = new Set(["Task", "Agent"]);

export function toolVisualKind(toolName: string): SessionToolKind {
  const name = resolveRegisteredToolName(toolName);
  if (COMMAND_TOOLS.has(name)) return "command";
  if (EDIT_TOOLS.has(name)) return "edit";
  if (READ_TOOLS.has(name)) return "read";
  if (SEARCH_TOOLS.has(name)) return "search";
  if (WEB_TOOLS.has(name)) return "web";
  if (AGENT_TOOLS.has(name)) return "agent";
  return "other";
}

export function toolVisualStatus(input: SessionToolVisualInput): SessionToolVisualStatus {
  if (input.status === "running") return "live";
  if (input.status === "interrupted") return "stopped";
  if (input.isError || input.resultFailed) {
    return input.turnFailed ? "failed" : "absorbed";
  }
  return input.resultEmpty ? "empty" : "settled";
}

export function toolVisualState(
  toolName: string,
  input: SessionToolVisualInput
): SessionToolVisualState {
  const status = toolVisualStatus(input);
  return { status, kind: toolVisualKind(toolName), statusLabel: STATUS_LABEL[status] };
}

/** The live row sentence: verb from the kind, preview from the production summary extractor. */
export interface SessionLiveToolLabel {
  verb: string;
  preview: string | null;
  /** Full accessible sentence ("Running shell — go test ./..."). */
  text: string;
}

const LIVE_VERB: Record<SessionToolKind, string> = {
  command: "Running shell",
  edit: "Editing",
  read: "Reading",
  search: "Searching",
  web: "Fetching",
  agent: "Running agent",
  other: "Running",
};

/** Chooses an activity verb from canonical tool identity rather than provider prose. */
function liveVerb(kind: SessionToolKind, registryTool: string): string {
  if (kind === "search") return registryTool === "Glob" ? "Finding files" : "Searching content";
  if (kind === "web") return registryTool === "WebSearch" ? "Searching the web" : "Fetching";
  if (kind === "other") return `Running ${toolHeadingName(registryTool)}`;
  return LIVE_VERB[kind];
}

/** Builds a bounded input preview, including an agent's description or prompt. */
function livePreview(
  kind: SessionToolKind,
  registryTool: string,
  args: Record<string, unknown>
): string | null {
  if (kind === "agent") {
    const description = args.description ?? args.prompt;
    return typeof description === "string" && description.trim()
      ? compactSessionSummary(description)
      : null;
  }
  const summary = getToolCompactSummary(registryTool, args);
  return summary && summary.trim().length > 0 ? summary : null;
}

/** Combines the canonical action with a bounded provider title or input preview. */
export function liveToolLabel(
  toolName: string,
  args: Record<string, unknown> = {},
  title?: string
): SessionLiveToolLabel {
  const registryTool = resolveRegisteredToolName(toolName);
  const kind = toolVisualKind(toolName);
  const verb = liveVerb(kind, registryTool);
  const description =
    title && title !== registryTool
      ? title
      : toolHeadingName(toolName) === "tool" && toolName !== "tool"
        ? toolName
        : null;
  const preview = description
    ? compactSessionSummary(description)
    : livePreview(kind, registryTool, args);
  if (preview === null) return { verb, preview, text: verb };
  // Paths read as the object of the verb ("Editing a/b.ts"); everything else is
  // set off with a dash ("Running shell — go test").
  const joined = kind === "edit" || kind === "read" ? `${verb} ${preview}` : `${verb} — ${preview}`;
  return { verb, preview, text: joined };
}

/** The parallel row sentence: one honest count, never one row per call. */
export function parallelToolLabel(count: number): string {
  return `Running ${count} tools…`;
}
