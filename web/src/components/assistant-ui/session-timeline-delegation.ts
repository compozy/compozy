/**
 * Delegation parts stay visible. They never fold into "Used N tools", and
 * consecutive settled asks stack instead of disappearing behind a summary.
 */
import {
  isAgentCallToolName,
  isCallReturnToolName,
} from "@/systems/agent-comms/lib/agent-call-tool-parts";

import type { SessionTimelineToolPart } from "./session-timeline.logic";

export type WorkSegmentKind = "work" | "delegation" | "return";

export interface WorkSegment {
  kind: WorkSegmentKind;
  tools: SessionTimelineToolPart[];
}

export function isDelegationToolPart(part: SessionTimelineToolPart): boolean {
  return isAgentCallToolName(part.toolName);
}

export function isCallReturnToolPart(part: SessionTimelineToolPart): boolean {
  return isCallReturnToolName(part.toolName);
}

export function isSettledDelegationStack(tools: readonly SessionTimelineToolPart[]): boolean {
  return (
    tools.length >= 2 &&
    tools.every(tool => isDelegationToolPart(tool) && tool.status === "settled")
  );
}

export function splitWorkAndDelegation(tools: readonly SessionTimelineToolPart[]): WorkSegment[] {
  const segments: WorkSegment[] = [];
  let current: WorkSegment | null = null;

  const flush = () => {
    if (current && current.tools.length > 0) segments.push(current);
    current = null;
  };

  for (const tool of tools) {
    if (isCallReturnToolPart(tool)) {
      flush();
      segments.push({ kind: "return", tools: [tool] });
      continue;
    }
    const kind: WorkSegmentKind = isDelegationToolPart(tool) ? "delegation" : "work";
    if (current === null || current.kind !== kind) {
      flush();
      current = { kind, tools: [tool] };
      continue;
    }
    current.tools.push(tool);
  }
  flush();
  return segments;
}
