import {
  Activity,
  Blocks,
  Bot,
  Boxes,
  Brain,
  FileEdit,
  FileText,
  FolderSearch,
  Globe,
  Hammer,
  LibraryBig,
  Lightbulb,
  ListChecks,
  Map,
  MessageCircleQuestion,
  NotebookPen,
  PackageSearch,
  Plug,
  Radio,
  Repeat,
  ScrollText,
  Search,
  SlidersHorizontal,
  Sparkles,
  Terminal,
  Workflow,
  Wrench,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

// --- Tool Icons ---

const TOOL_ICONS: Record<string, LucideIcon> = {
  Bash: Terminal,
  Read: FileText,
  Write: FileEdit,
  Edit: FileEdit,
  Grep: Search,
  Glob: FolderSearch,
  WebSearch: Globe,
  WebFetch: Globe,
  Task: Hammer,
  Agent: Hammer,
  Think: Lightbulb,
  TodoWrite: ListChecks,
  NotebookEdit: NotebookPen,
  EnterPlanMode: Lightbulb,
  ExitPlanMode: Map,
  AskUserQuestion: MessageCircleQuestion,
  ToolSearch: PackageSearch,
  Skill: Sparkles,
};

/**
 * Icon per AGH native tool family (`agh__<family>_<verb>` → icon). Derived from
 * AGH's own `agh__*` tool taxonomy. Unmapped families fall through to the
 * generic tool fallback.
 */
const AGH_NATIVE_FAMILY_ICONS: Record<string, LucideIcon> = {
  edit: FileEdit,
  memory: Brain,
  network: Radio,
  config: SlidersHorizontal,
  automation: Workflow,
  loop: Repeat,
  loops: Repeat,
  agent: Bot,
  observe: Activity,
  logs: ScrollText,
  bundles: Boxes,
  extensions: Blocks,
  catalog: LibraryBig,
};

const AGH_NATIVE_PREFIX = "agh__";
const MCP_PREFIX = "mcp__";

/**
 * Extract the family segment of an AGH native tool id (`agh__memory_note` →
 * `memory`). Returns null for non-native ids or an empty family segment.
 */
function aghNativeFamily(toolName: string): string | null {
  if (!toolName.startsWith(AGH_NATIVE_PREFIX)) return null;
  const family = toolName.slice(AGH_NATIVE_PREFIX.length).split("_", 1)[0] ?? "";
  return family.length > 0 ? family : null;
}

/**
 * Resolve a tool icon by name. Order: exact builtin map → MCP bridge tools →
 * AGH native tool family → semantic input fallbacks → generic tool glyph.
 */
export function getToolIcon(toolName: string, toolInput?: Record<string, unknown>): LucideIcon {
  const direct = TOOL_ICONS[toolName];
  if (direct) return direct;

  if (toolName.startsWith(MCP_PREFIX)) return Plug;

  const family = aghNativeFamily(toolName);
  if (family) {
    const familyIcon = AGH_NATIVE_FAMILY_ICONS[family];
    if (familyIcon) return familyIcon;
  }

  // Semantic fallbacks for uncatalogued dynamic tools.
  if (toolInput) {
    if ("command" in toolInput) return Terminal;
    if ("file_path" in toolInput || "filePath" in toolInput) return FileText;
    if ("pattern" in toolInput) return Search;
    if ("url" in toolInput || "query" in toolInput) return Globe;
  }

  return Wrench;
}

// --- Tool Labels ---

export type ToolLabelTense = "active" | "past" | "failure";

interface ToolLabels {
  active: string;
  past: string;
  failure: string;
}

const TOOL_LABELS: Record<string, ToolLabels> = {
  Bash: { active: "Running...", past: "Ran command", failure: "run command" },
  Read: { active: "Reading...", past: "Read file", failure: "read file" },
  Write: { active: "Writing...", past: "Wrote file", failure: "write file" },
  Edit: { active: "Editing...", past: "Edited file", failure: "edit file" },
  Grep: { active: "Searching...", past: "Searched content", failure: "search content" },
  Glob: { active: "Finding files...", past: "Found files", failure: "find files" },
  WebSearch: { active: "Searching web...", past: "Searched web", failure: "search web" },
  WebFetch: { active: "Fetching page...", past: "Fetched page", failure: "fetch page" },
  Task: { active: "Running task...", past: "Ran task", failure: "run task" },
  Agent: { active: "Running agent...", past: "Ran agent", failure: "run agent" },
  Think: { active: "Thinking...", past: "Thought", failure: "think" },
  TodoWrite: { active: "Updating tasks...", past: "Updated tasks", failure: "update tasks" },
  NotebookEdit: {
    active: "Editing notebook...",
    past: "Edited notebook",
    failure: "edit notebook",
  },
  EnterPlanMode: {
    active: "Entering plan mode...",
    past: "Entered plan mode",
    failure: "enter plan mode",
  },
  ExitPlanMode: {
    active: "Preparing plan...",
    past: "Presented plan",
    failure: "prepare plan",
  },
  AskUserQuestion: { active: "Asking...", past: "Asked question", failure: "ask question" },
  ToolSearch: { active: "Loading tools...", past: "Loaded tools", failure: "load tools" },
  Skill: { active: "Loading skill...", past: "Loaded skill", failure: "load skill" },
};

const REGISTERED_TOOL_NAMES = new Set(Object.keys(TOOL_LABELS));

/**
 * Resolve a canonical registry tool id from a streamed or persisted tool name.
 * Accepts exact ids ("Read") and ACP-style titles ("Read routes.go").
 */
export function resolveRegisteredToolName(toolName: string): string {
  const trimmed = toolName.trim();
  if (trimmed === "") {
    return "tool";
  }
  if (REGISTERED_TOOL_NAMES.has(trimmed)) {
    return trimmed;
  }
  for (const registered of REGISTERED_TOOL_NAMES) {
    if (trimmed.startsWith(`${registered} `)) {
      return registered;
    }
  }
  return trimmed;
}

export function getToolLabel(toolName: string, tense: ToolLabelTense): string {
  const labels = TOOL_LABELS[toolName];
  if (labels) return labels[tense];

  // Fallback for unknown tools
  switch (tense) {
    case "active":
      return `Running ${toolName}...`;
    case "past":
      return `Used ${toolName}`;
    case "failure":
      return `use ${toolName}`;
  }
}

// --- Compact Summary Extractors ---

/**
 * Extract a short summary string from tool input for the collapsed tool-call row preview.
 * Returns undefined if no meaningful summary can be extracted.
 */
export function getToolCompactSummary(
  toolName: string,
  toolInput?: Record<string, unknown>
): string | undefined {
  const fullSummary = getToolFullSummary(toolName, toolInput);
  if (fullSummary === undefined) return undefined;

  return truncate(fullSummary, getToolSummaryMaxLength(toolName));
}

function truncate(str: string, maxLen: number): string {
  if (!str) return "";
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen - 1) + "\u2026";
}

function getToolFullSummary(
  toolName: string,
  toolInput?: Record<string, unknown>
): string | undefined {
  if (!toolInput) return undefined;

  switch (toolName) {
    case "Bash":
      return String(toolInput.command ?? "");
    case "Read":
    case "Write":
    case "Edit":
      return String(toolInput.file_path ?? toolInput.filePath ?? "");
    case "Grep":
    case "Glob":
      return String(toolInput.pattern ?? "");
    case "WebSearch":
      return String(toolInput.query ?? "");
    case "WebFetch":
      return String(toolInput.url ?? "");
    default:
      return undefined;
  }
}

function getToolSummaryMaxLength(toolName: string): number {
  return toolName === "Bash" ? 80 : 60;
}
