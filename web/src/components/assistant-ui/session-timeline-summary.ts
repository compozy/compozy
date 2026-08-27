// Semantic summary of a settled tool run — the collapsed `.tgroup-sum` label
// ("Ran 2 commands · Edited 3 files"). Port of synara's `toolCallGroup.logic.ts`
// summarizer onto `SessionTimelineToolPart`: fixed category order, distinct-file
// counts for Edited/Read, and a 2+ minimum before a run may fold. CompozyOS's data
// layer stays authoritative — categories derive from the registered tool name,
// file identity from the tool args the renderers already read.

import {
  isAgentCallToolName,
  isCallReturnToolName,
} from "@/systems/agent-comms/lib/agent-call-tool-parts";
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
  /** Middle-dot–joined sentence, e.g. "Ran 2 commands · Edited 3 files". */
  label: string;
  parts: SessionToolGroupSummaryPart[];
  entryCount: number;
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
 * Only a settled, non-failed call may disappear into a summary count. Failures
 * and interruptions stay individually visible (glyph + preview) and running
 * calls belong to the live tail — a summary must never hide either.
 */
export function isSummarizableToolPart(part: SessionTimelineToolPart): boolean {
  return (
    part.status === "settled" &&
    part.isError !== true &&
    !isAgentCallToolName(part.toolName) &&
    !isCallReturnToolName(part.toolName)
  );
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
    label: parts.map(part => part.label).join(" · "),
    parts,
    entryCount: summarizable.length,
  };
}
