/**
 * Live-tail and settled work-row derivation, including delegation isolation.
 */
import { splitWorkAndDelegation } from "./session-timeline-delegation";
import { workGroupId } from "./session-timeline-group-identity";
import {
  isSummarizableToolPart,
  MIN_COLLAPSIBLE_TOOL_GROUP_SIZE,
  summarizeToolGroup,
  type SessionToolGroupSummary,
} from "./session-timeline-summary";
import type {
  DeriveSessionRowsOptions,
  SessionRow,
  SessionTimelinePart,
  SessionTimelineToolPart,
  SessionWorkRow,
} from "./session-timeline.logic";

const ACTIVE_WORK_VISIBLE_LIMIT = 4;

export function findLiveTailClusterStart(
  parts: readonly SessionTimelinePart[],
  activeTurnId: string | undefined
): string | null {
  let index = parts.length - 1;
  while (index >= 0 && parts[index]!.kind === "working") index -= 1;
  const last = index >= 0 ? parts[index] : undefined;
  if (!last || last.kind !== "tool") return null;
  const cluster: SessionTimelineToolPart[] = [last];
  for (let scan = index - 1; scan >= 0; scan -= 1) {
    const previous = parts[scan]!;
    if (previous.kind !== "tool" || previous.turnId !== last.turnId) break;
    cluster.unshift(previous);
  }
  const hasRunning = cluster.some(tool => tool.status === "running");
  const activeTurn =
    activeTurnId !== undefined && activeTurnId !== "" && last.turnId === activeTurnId;
  return hasRunning || activeTurn ? (cluster[0]?.id ?? null) : null;
}

export function workRowsFromCluster(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  liveTailStartId: string | null,
  usedGroupIds: Set<string>
): SessionRow[] {
  const first = tools[0];
  if (!first) return [];
  return splitWorkAndDelegation(tools).flatMap(segment => {
    if (segment.kind === "delegation") {
      return delegationWorkRows(segment.tools, options, usedGroupIds);
    }
    if (segment.kind === "return") {
      return segment.tools.map(tool => {
        const groupId = workGroupId([tool], { ...options, usedGroupIds });
        usedGroupIds.add(groupId);
        return settledWorkRow([tool], null, groupId, options);
      });
    }
    const live =
      segment.tools[0]?.id === liveTailStartId ||
      segment.tools.some(tool => tool.status === "running");
    return live
      ? liveWorkRows(segment.tools, options, usedGroupIds)
      : settledWorkRows(segment.tools, options, usedGroupIds);
  });
}

function delegationWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  usedGroupIds: Set<string>
): SessionRow[] {
  const settled = tools.filter(tool => tool.status === "settled");
  const live = tools.filter(tool => tool.status !== "settled");
  const rows: SessionRow[] = [];
  if (settled.length >= 2) {
    const groupId = workGroupId(settled, { ...options, usedGroupIds });
    usedGroupIds.add(groupId);
    rows.push(settledWorkRow(settled, null, groupId, options));
  } else {
    for (const tool of settled) {
      const groupId = workGroupId([tool], { ...options, usedGroupIds });
      usedGroupIds.add(groupId);
      rows.push(settledWorkRow([tool], null, groupId, options));
    }
  }
  for (const tool of live) {
    const groupId = workGroupId([tool], { ...options, usedGroupIds });
    usedGroupIds.add(groupId);
    rows.push(settledWorkRow([tool], null, groupId, options));
  }
  return rows;
}

function liveWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  usedGroupIds: Set<string>
): SessionRow[] {
  const first = tools[0]!;
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
    const summarizable = isSummarizableToolPart(tool);
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
  summary: SessionToolGroupSummary | null,
  groupId: string,
  options: DeriveSessionRowsOptions
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
    active: false,
  };
}
