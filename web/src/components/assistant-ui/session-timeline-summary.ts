// Semantic summary of a settled tool run — the group sentence ("Ran 6 commands,
// edited 2 files, read 3 files") that stands in for the rows behind a
// completed-tools group (ADR-006) or a settled-turn fold. Fixed category order,
// distinct-file counts for edits and reads, and a 2+ minimum before a run may
// collapse. A failure the agent absorbed stays inside the group as information
// ("· 1 failed", ADR-009): it is counted in its category and in `failedCount`,
// never promoted to an alarm. CompozyOS's data layer stays authoritative —
// categories derive from the same tool display the rows show, file identity
// from the tool args the renderers already read.

import { isDeliberateTerminalTool } from "@/systems/session/lib/session-terminal-tools";
import { resolveToolDisplay } from "@/systems/session/lib/tool-display";
import { leadingCdDir } from "@/systems/session/lib/tool-shell-intent";
import {
  getToolLabel,
  isSubagentToolName,
  resolveRegisteredToolName,
} from "@/systems/session/lib/tool-labels";

import type { SessionTimelineToolPart } from "./session-timeline.logic";

/** A single tool row collapses into nothing useful; only runs of 2+ fold. */
export const MIN_COLLAPSIBLE_TOOL_GROUP_SIZE = 2;

export type SessionToolSummaryCategory =
  | "command"
  | "edit"
  | "read"
  | "list"
  | "search"
  | "subagent"
  | "tool";

export interface SessionToolGroupSummaryPart {
  category: SessionToolSummaryCategory;
  count: number;
  label: string;
}

export interface SessionToolGroupSummary {
  /** Clauses joined by " · ", e.g. "Ran 2 commands · Edited 3 files". */
  label: string;
  parts: SessionToolGroupSummaryPart[];
  entryCount: number;
  /** Absorbed failures inside the group; the row appends "· N failed" in the same ink. */
  failedCount: number;
}

/** Fixed presentation order: Ran → Edited → Read → Listed → Searched → subagent tools → Used. */
const CATEGORY_ORDER: readonly SessionToolSummaryCategory[] = [
  "command",
  "edit",
  "read",
  "list",
  "search",
  "subagent",
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
  if (isSubagentToolName(name)) return "subagent";
  // The same kind the row shows: a title-named shell tool counts as a command,
  // a `cat`/`rg` command as the read or search it performs.
  const display = resolveToolDisplay({ toolName: part.toolName, args: part.args }, "past");
  switch (display.kind) {
    case "command":
      return "command";
    case "edit":
      return "edit";
    case "read":
      return display.listing ? "list" : "read";
    case "search":
    case "web":
      return "search";
    default:
      return "tool";
  }
}

// Distinct-file identity for an edit/read entry: the full path its row names,
// whether a tool argument or a shell read carried it (`Read a.ts` and `cat a.ts`
// are one file). A relative shell path resolves against its leading `cd`, so
// `package.json` read in two directories stays two files. Entries with no file
// info count as one unit each so the total never under-reports work.
function entryFileKeys(part: SessionTimelineToolPart): string[] {
  const display = resolveToolDisplay({ toolName: part.toolName, args: part.args }, "past");
  const raw = display.targetKind === "file" ? display.fullTarget : undefined;
  if (typeof raw !== "string") return [];
  const trimmed = raw.trim();
  if (trimmed.length === 0) return [];
  const cwd = typeof part.args.command === "string" ? leadingCdDir(part.args.command) : null;
  return [cwd && !trimmed.startsWith("/") ? `${cwd.replace(/\/+$/u, "")}/${trimmed}` : trimmed];
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
    case "list":
      return `Listed ${count} ${count === 1 ? "directory" : "directories"}`;
    case "search":
      return count === 1 ? "Searched once" : `Searched ${count} times`;
    case "subagent":
      return `Used ${count} subagent ${pluralNoun(count, "tool")}`;
    case "tool":
      return `Used ${count} ${pluralNoun(count, "tool")}`;
  }
}

// One line of clauses, each keeping its capital, joined by the same " · " the
// fold uses — "Ran 6 commands · Edited 2 files · Read app.ts".
function joinSummaryLabels(labels: readonly string[]): string {
  return labels.join(" · ");
}

function fileLeaf(path: string): string {
  return (
    path
      .split("/")
      .filter(segment => segment.length > 0)
      .at(-1) ?? path
  );
}

// A clause that touched exactly one file names it: "Edited styles.css".
function singleFileLabel(
  category: SessionToolSummaryCategory,
  files: ReadonlySet<string> | undefined,
  count: number
): string | null {
  if (count !== 1 || files?.size !== 1) return null;
  const [path] = files;
  if (path === undefined) return null;
  if (category === "edit") return `Edited ${fileLeaf(path)}`;
  if (category === "read") return `Read ${fileLeaf(path)}`;
  if (category === "list") return `Listed ${fileLeaf(path)}`;
  return null;
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
    if (category === "edit" || category === "read" || category === "list") {
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

  const parts = CATEGORY_ORDER.flatMap(category => {
    if (category === "subagent") return subagentSummaryParts(summarizable);
    const count = countByCategory.get(category) ?? 0;
    if (count === 0) return [];
    const label =
      singleFileLabel(category, distinctFilesByCategory.get(category), count) ??
      summaryPartLabel(category, count);
    return [{ category, count, label }];
  });

  return {
    label: joinSummaryLabels(parts.map(part => part.label)),
    parts,
    entryCount: summarizable.length,
    failedCount: summarizable.filter(isAbsorbedToolFailure).length,
  };
}

/**
 * Subagent tools speak per tool, in their own verb (S6, UT-W09, COPY.md
 * "Subagent Terms"): repeats add "… 2 times"; a tool whose every call failed
 * reads "Tried to check subagent capabilities".
 */
function subagentSummaryParts(
  parts: readonly SessionTimelineToolPart[]
): SessionToolGroupSummaryPart[] {
  const byTool = new Map<string, { count: number; succeeded: number }>();
  for (const part of parts) {
    const name = resolveRegisteredToolName(part.toolName);
    if (!isSubagentToolName(name)) continue;
    const entry = byTool.get(name) ?? { count: 0, succeeded: 0 };
    entry.count += 1;
    if (!isAbsorbedToolFailure(part)) entry.succeeded += 1;
    byTool.set(name, entry);
  }
  return [...byTool].map(([name, { count, succeeded }]) => ({
    category: "subagent" as const,
    count,
    label:
      succeeded === 0
        ? `Tried to ${getToolLabel(name, "failure")}`
        : count === 1
          ? getToolLabel(name, "past")
          : `${getToolLabel(name, "past")} ${count} times`,
  }));
}

/** The "· N failed" suffix a group or fold appends after its sentence; `null` when nothing failed. */
export function summaryFailureSuffix(summary: SessionToolGroupSummary | null): string | null {
  if (!summary || summary.failedCount === 0) return null;
  return `${summary.failedCount} failed`;
}
