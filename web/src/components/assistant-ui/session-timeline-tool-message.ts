import { resolveToolResult, type UIMessage } from "@/systems/session";

import type { SessionTimelineToolPart } from "./session-timeline.logic";

/** The `UIMessage` shape `SessionToolCallRow` renders, from a derived tool part. */
export function toolMessageFromPart(part: SessionTimelineToolPart): UIMessage {
  return {
    id: part.toolCallId,
    role: part.result !== undefined || part.isError ? "tool_result" : "tool_call",
    content: "",
    toolName: part.toolName,
    toolTitle: part.toolTitle,
    toolInput: part.args,
    toolResult: resolveToolResult(part.result),
    toolError: part.isError,
    isStreaming: part.status === "running",
    timestamp: part.timestamp ? Date.parse(part.timestamp) : Date.now(),
  };
}
