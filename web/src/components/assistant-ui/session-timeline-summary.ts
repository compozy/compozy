// Semantic summary of a settled tool run — the group sentence ("Ran 6 commands,
// edited 2 files, read 3 files") that stands in for the rows behind a
// completed-tools group (ADR-006) or a settled-turn fold. Fixed category order,
// distinct-file counts for edits and reads, and a 2+ minimum before a run may
// collapse. A failure the agent absorbed stays inside the group as information
// ("· 1 failed", ADR-009): it is counted in its category and in `failedCount`,
// never promoted to an alarm. CompozyOS's data layer stays authoritative —
// categories derive from the registered tool name, file identity from the tool
// args the renderers already read.

import { isDeliberateTerminalTool } from "@/systems/session/lib/session-terminal-tools";
import { resolveRegisteredToolName } from "@/systems/session/lib/tool-labels";

import type { SessionTimelineToolPart } from "./session-timeline.logic";

/** A single tool row collapses into nothing useful; only runs of 2+ fold. */
export const MIN_COLLAPSIBLE_TOOL_GROUP_SIZE = 2;

export type SessionToolSummaryCategory = "command" | "edit" | "read" | "search" | "agent" | "tool";

export interface SessionToolGroupSummaryPart {
  category: SessionToolSummaryCategory;
  count: number;
  label: string;
}

export interface SessionToolGroupSummary {
  /** Comma-joined sentence, e.g. "Ran 2 commands, edited 3 files". */
  label: string;
  parts: SessionToolGroupSummaryPart[];
  entryCount: number;
  /** Absorbed failures inside the group; the row appends "· N failed" in the same ink. */
  failedCount: number;
}

const COMMAND_TOOLS = new Set(["Bash"]);
const EDIT_TOOLS = new Set(["Edit", "Write", "NotebookEdit"]);
const READ_TOOLS = new Set(["Read"]);
const SEARCH_TOOLS = new Set(["Grep", "Glob", "WebSearch", "WebFetch"]);
const AGENT_TOOLS = new Set(["Task", "Agent"]);

/** Fixed presentation order: Ran → Edited → Read → Searched → agent tasks → Used. */
const CATEGORY_ORDER: readonly SessionToolSummaryCategory[] = [
  "command",
  "edit",
  "read",
  "search",
  "agent",
  "tool",
];

/**
 * Only a settled call may disappear into a summary count. Running calls belong
 * to the live row, interrupted calls stay individually visible in the open
 * turn, and deliberate terminal blocks are their own surface. A settled call
 * that failed still counts: the agent kept going, so the group says so.
 */
export function isSummarizableToolPart(part: SessionTimelineToolPart): boolean {
  return part.status === "settled" && !isDeliberateTerminalTool(part.toolName);
}

/** A settled call whose result was an error the turn absorbed (ADR-009 "absorbed failure"). */
export function isAbsorbedToolFailure(part: SessionTimelineToolPart): boolean {
  return part.status === "settled" && part.isError === true;
}

export function classifyToolSummaryCategory(
  part: SessionTimelineToolPart
): SessionToolSummaryCategory {
  const name = resolveRegisteredToolName(part.toolName);
  if (COMMAND_TOOLS.has(name)) return "command";
  if (EDIT_TOOLS.has(name)) return "edit";
  if (READ_TOOLS.has(name)) return "read";
  if (SEARCH_TOOLS.has(name)) return "search";
  if (AGENT_TOOLS.has(name)) return "agent";
  return "tool";
}

// Distinct-file identity for an edit/read entry. Entries with no file info
// count as one unit each so the total never under-reports work.
function entryFileKeys(part: SessionTimelineToolPart): string[] {
  const raw = part.args.file_path ?? part.args.filePath ?? part.args.notebook_path;
  if (typeof raw !== "string") return [];
  const trimmed = raw.trim();
  return trimmed.length > 0 ? [trimmed] : [];
}

function pluralNoun(count: number, singular: string): string {
  return count === 1 ? singular : `${singular}s`;
}

function summaryPartLabel(category: SessionToolSummaryCategory, count: number): string {
  switch (category) {
    case "command":
      return `Ran ${count} ${pluralNoun(count, "command")}`;
    case "edit":
      return `Edited ${count} ${pluralNoun(count, "file")}`;
    case "read":
      return `Read ${count} ${pluralNoun(count, "file")}`;
    case "search":
      return `Searched ${count} ${pluralNoun(count, "file")}`;
    case "agent":
      return `Ran ${count} agent ${pluralNoun(count, "task")}`;
    case "tool":
      return `Used ${count} ${pluralNoun(count, "tool")}`;
  }
}

// One sentence: the first label keeps its capital, the rest continue in lower
// case, joined by commas — "Ran 6 commands, edited 2 files, read 3 files".
function joinSummaryLabels(labels: readonly string[]): string {
  return labels
    .map((label, index) => (index === 0 ? label : label.charAt(0).toLowerCase() + label.slice(1)))
    .join(", ");
}

/**
 * Summarize a settled run into category counts, or `null` when fewer than
 * `MIN_COLLAPSIBLE_TOOL_GROUP_SIZE` entries are summarizable. Edited/Read count
 * distinct files; every other category counts calls.
 */
export function summarizeToolGroup(
  entries: readonly SessionTimelineToolPart[]
): SessionToolGroupSummary | null {
  const summarizable = entries.filter(isSummarizableToolPart);
  if (summarizable.length < MIN_COLLAPSIBLE_TOOL_GROUP_SIZE) {
    return null;
  }

  const countByCategory = new Map<SessionToolSummaryCategory, number>();
  const distinctFilesByCategory = new Map<SessionToolSummaryCategory, Set<string>>();

  for (const part of summarizable) {
    const category = classifyToolSummaryCategory(part);
    if (category === "edit" || category === "read") {
      const fileKeys = entryFileKeys(part);
      if (fileKeys.length === 0) {
        countByCategory.set(category, (countByCategory.get(category) ?? 0) + 1);
        continue;
      }
      let distinctFiles = distinctFilesByCategory.get(category);
      if (!distinctFiles) {
        distinctFiles = new Set();
        distinctFilesByCategory.set(category, distinctFiles);
      }
      for (const fileKey of fileKeys) {
        distinctFiles.add(fileKey);
      }
      continue;
    }
    countByCategory.set(category, (countByCategory.get(category) ?? 0) + 1);
  }

  for (const [category, distinctFiles] of distinctFilesByCategory) {
    countByCategory.set(category, (countByCategory.get(category) ?? 0) + distinctFiles.size);
  }

  const parts = CATEGORY_ORDER.filter(category => (countByCategory.get(category) ?? 0) > 0).map(
    category => {
      const count = countByCategory.get(category)!;
      return { category, count, label: summaryPartLabel(category, count) };
    }
  );

  return {
    label: joinSummaryLabels(parts.map(part => part.label)),
    parts,
    entryCount: summarizable.length,
    failedCount: summarizable.filter(isAbsorbedToolFailure).length,
  };
}

/** The "· N failed" suffix a group or fold appends after its sentence; `null` when nothing failed. */
export function summaryFailureSuffix(summary: SessionToolGroupSummary | null): string | null {
  if (!summary || summary.failedCount === 0) return null;
  return `${summary.failedCount} failed`;
}
