// Same-turn work projection. Mixed activity retains chronological order in one
// disclosure; tool-only activity keeps the existing live/settled presentation.

import { isDeliberateTerminalTool } from "@/systems/session/lib/session-terminal-tools";

import { isStreamingState, reasoningRowFromCluster } from "./session-timeline.logic";
import { workGroupId } from "./session-timeline-group-identity";
import {
  classifyToolSummaryCategory,
  MIN_COLLAPSIBLE_TOOL_GROUP_SIZE,
  summarizeToolGroup,
} from "./session-timeline-summary";
import type {
  DeriveSessionRowsOptions,
  SessionLiveToolRow,
  SessionRow,
  SessionTimelineToolPart,
  SessionWorkRow,
  SessionWorkEntry,
} from "./session-timeline.logic";

/** Tool-only live runs keep each child agent on its own row. */
export function isAgentToolPart(part: SessionTimelineToolPart): boolean {
  return classifyToolSummaryCategory(part) === "agent";
}

/** Stable identity of the turn's one live row so its motion survives tool swaps. */
export function liveToolRowId(turnId: string | undefined): string {
  return `live:${turnId ?? "none"}`;
}

/** Keep mixed activity ordered while leaving deliberate terminal interactions visible. */
export function workRowsFromCluster(
  entries: SessionWorkEntry[],
  options: DeriveSessionRowsOptions,
  liveTailStartId: string | null,
  usedGroupIds: Set<string>
): SessionRow[] {
  const rows: SessionRow[] = [];
  let segment: SessionWorkEntry[] = [];
  const flush = () => {
    if (segment.length === 0) return;
    const tools = segment.filter(entry => entry.kind === "tool");
    const reasoning = segment.filter(entry => entry.kind === "reasoning");
    if (tools.length === 0) {
      rows.push(reasoningRowFromCluster(reasoning));
    } else if (reasoning.length === 0) {
      rows.push(...toolRowsFromCluster(tools, options, liveTailStartId, usedGroupIds));
    } else {
      const groupId = workGroupId(segment, { ...options, usedGroupIds });
      usedGroupIds.add(groupId);
      const running = tools.filter(tool => tool.status === "running").length;
      const stopped = tools.filter(tool => tool.status === "interrupted").length;
      const thinking = reasoning.some(part => isStreamingState(part.state));
      const label = [
        `${tools.length} ${tools.length === 1 ? "tool" : "tools"}`,
        `${reasoning.length} ${reasoning.length === 1 ? "thought" : "thoughts"}`,
        ...(running ? [`${running} running`] : []),
        ...(thinking ? ["thinking"] : []),
        ...(stopped ? [`${stopped} stopped`] : []),
      ].join(" · ");
      rows.push(
        settledWorkRow(
          segment,
          {
            label,
            parts: [],
            entryCount: tools.length,
            failedCount: tools.filter(tool => tool.isError).length,
          },
          groupId,
          options,
          running > 0 || thinking
        )
      );
    }
    segment = [];
  };
  for (const entry of entries) {
    if (entry.kind === "tool" && isDeliberateTerminalTool(entry.toolName)) {
      flush();
      rows.push(...toolRowsFromCluster([entry], options, liveTailStartId, usedGroupIds));
    } else {
      segment.push(entry);
    }
  }
  flush();
  return rows;
}

/** Retain tool-only live and settled presentation within each terminal boundary. */
function toolRowsFromCluster(
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

/**
 * The live tail: completed calls first (one group when 2+ summarize, otherwise
 * their own rows), then one live row per running child agent, then the single
 * live row for every other running call. Order inside the narrative reads
 * "what it already did, what it's doing".
 */
function liveWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  usedGroupIds: Set<string>
): SessionRow[] {
  const first = tools[0];
  if (!first) return [];
  const completed = tools.filter(tool => tool.status !== "running");
  const running = tools.filter(tool => tool.status === "running");
  const rows: SessionRow[] = [];

  if (completed.length > 0) {
    for (const chunk of chunkSettled(completed)) {
      const summary =
        chunk.summarizable && chunk.entries.length >= MIN_COLLAPSIBLE_TOOL_GROUP_SIZE
          ? summarizeToolGroup(chunk.entries)
          : null;
      const groupId = workGroupId(chunk.entries, { ...options, usedGroupIds });
      usedGroupIds.add(groupId);
      rows.push(settledWorkRow(chunk.entries, summary, groupId, options, true));
    }
  }

  const agents = running.filter(isAgentToolPart);
  const parallel = running.filter(tool => !isAgentToolPart(tool));
  for (const agent of agents) {
    rows.push({
      kind: "live-tool",
      id: `live-agent:${agent.toolCallId.trim() || agent.id}`,
      turnId: agent.turnId,
      timestamp: agent.timestamp,
      entries: [agent],
      agent: true,
      expanded: false,
    });
  }
  if (parallel.length > 0) {
    const id = liveToolRowId(first.turnId);
    rows.push({
      kind: "live-tool",
      id,
      turnId: first.turnId,
      timestamp: parallel[0]?.timestamp,
      entries: parallel,
      agent: false,
      expanded: parallel.length > 1 ? (options.expandedWorkGroupIds?.has(id) ?? false) : false,
    } satisfies SessionLiveToolRow);
  }
  return rows;
}

// Summarizable calls (settled, terminal blocks aside) and the rest (interrupted
// calls stay visible one by one) alternate as chunks so order is preserved.
function chunkSettled(
  tools: SessionTimelineToolPart[]
): { summarizable: boolean; entries: SessionTimelineToolPart[] }[] {
  const chunks: { summarizable: boolean; entries: SessionTimelineToolPart[] }[] = [];
  for (const tool of tools) {
    const summarizable = tool.status === "settled" && !isDeliberateTerminalTool(tool.toolName);
    const lastChunk = chunks.at(-1);
    if (lastChunk && lastChunk.summarizable === summarizable) {
      lastChunk.entries.push(tool);
    } else {
      chunks.push({ summarizable, entries: [tool] });
    }
  }
  return chunks;
}

function settledWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  usedGroupIds: Set<string>
): SessionRow[] {
  return chunkSettled(tools).map(chunk => {
    const summary =
      chunk.summarizable && chunk.entries.length >= MIN_COLLAPSIBLE_TOOL_GROUP_SIZE
        ? summarizeToolGroup(chunk.entries)
        : null;
    const groupId = workGroupId(chunk.entries, { ...options, usedGroupIds });
    usedGroupIds.add(groupId);
    return settledWorkRow(chunk.entries, summary, groupId, options);
  });
}

/** Keep original entries and the retained group identity behind an optional summary. */
function settledWorkRow(
  entries: SessionWorkEntry[],
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
    expanded: summary ? (options.expandedWorkGroupIds?.has(groupId) ?? false) : false,
    active,
  };
}
