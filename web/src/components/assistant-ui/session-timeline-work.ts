import { isDeliberateTerminalTool } from "@/systems/session/lib/session-terminal-tools";

import { workGroupId } from "./session-timeline-group-identity";
import { MIN_COLLAPSIBLE_TOOL_GROUP_SIZE, summarizeToolGroup } from "./session-timeline-summary";
import type {
  DeriveSessionRowsOptions,
  SessionRow,
  SessionTimelineToolPart,
  SessionWorkRow,
} from "./session-timeline.logic";

const ACTIVE_WORK_VISIBLE_LIMIT = 4;

export function workRowsFromCluster(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  liveTailStartId: string | null,
  usedGroupIds: Set<string>
): SessionRow[] {
  const live = tools[0]?.id === liveTailStartId || tools.some(tool => tool.status === "running");
  const rows: SessionRow[] = [];
  for (const segment of splitTerminalSegments(tools)) {
    if (segment.terminal) {
      for (const entry of segment.entries) {
        const groupId = workGroupId([entry], { ...options, usedGroupIds });
        usedGroupIds.add(groupId);
        rows.push(settledWorkRow([entry], null, groupId, options, entry.status === "running"));
      }
      continue;
    }
    rows.push(
      ...(live
        ? liveWorkRows(segment.entries, options, usedGroupIds)
        : settledWorkRows(segment.entries, options, usedGroupIds))
    );
  }
  return rows;
}

function splitTerminalSegments(
  tools: SessionTimelineToolPart[]
): { terminal: boolean; entries: SessionTimelineToolPart[] }[] {
  const segments: { terminal: boolean; entries: SessionTimelineToolPart[] }[] = [];
  for (const tool of tools) {
    const terminal = isDeliberateTerminalTool(tool.toolName);
    const last = segments.at(-1);
    if (last && last.terminal === terminal && !terminal) {
      last.entries.push(tool);
      continue;
    }
    segments.push({ terminal, entries: [tool] });
  }
  return segments;
}

function liveWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  usedGroupIds: Set<string>
): SessionRow[] {
  const first = tools[0];
  if (!first) return [];
  const groupId = workGroupId(tools, { ...options, usedGroupIds });
  usedGroupIds.add(groupId);
  const grouped = tools.length > ACTIVE_WORK_VISIBLE_LIMIT;
  const expanded = grouped ? (options.expandedWorkGroupIds?.has(groupId) ?? false) : false;
  const workRow: SessionWorkRow = {
    kind: "work",
    id: groupId,
    groupId,
    turnId: first.turnId,
    timestamp: first.timestamp,
    entries: [...tools],
    summary: null,
    visibleCount: grouped ? ACTIVE_WORK_VISIBLE_LIMIT : tools.length,
    grouped,
    expanded,
    active: true,
  };
  if (!grouped) {
    return [workRow];
  }
  return [
    {
      kind: "work-toggle",
      id: `${groupId}:toggle`,
      groupId,
      turnId: first.turnId,
      timestamp: first.timestamp,
      hiddenCount: tools.length - ACTIVE_WORK_VISIBLE_LIMIT,
      expanded,
    },
    workRow,
  ];
}

function settledWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  usedGroupIds: Set<string>
): SessionRow[] {
  const chunks: { summarizable: boolean; entries: SessionTimelineToolPart[] }[] = [];
  for (const tool of tools) {
    const summarizable =
      tool.isError !== true &&
      tool.status !== "interrupted" &&
      !isDeliberateTerminalTool(tool.toolName);
    const lastChunk = chunks.at(-1);
    if (lastChunk && lastChunk.summarizable === summarizable) {
      lastChunk.entries.push(tool);
    } else {
      chunks.push({ summarizable, entries: [tool] });
    }
  }
  return chunks.map(chunk => {
    const summary =
      chunk.summarizable && chunk.entries.length >= MIN_COLLAPSIBLE_TOOL_GROUP_SIZE
        ? summarizeToolGroup(chunk.entries)
        : null;
    const groupId = workGroupId(chunk.entries, { ...options, usedGroupIds });
    usedGroupIds.add(groupId);
    return settledWorkRow(chunk.entries, summary, groupId, options);
  });
}

function settledWorkRow(
  entries: SessionTimelineToolPart[],
  summary: SessionWorkRow["summary"],
  groupId: string,
  options: DeriveSessionRowsOptions,
  active = false
): SessionWorkRow {
  const first = entries[0]!;
  return {
    kind: "work",
    id: groupId,
    groupId,
    turnId: first.turnId,
    timestamp: first.timestamp,
    entries: [...entries],
    summary,
    visibleCount: entries.length,
    grouped: false,
    expanded: summary ? (options.expandedWorkGroupIds?.has(groupId) ?? false) : false,
    active,
  };
}

/** Trailing entries shown while the live tail is collapsed. */
export function visibleWorkEntries(row: SessionWorkRow): SessionTimelineToolPart[] {
  if (!row.grouped || row.expanded) {
    return row.entries;
  }
  return row.entries.slice(-row.visibleCount);
}
