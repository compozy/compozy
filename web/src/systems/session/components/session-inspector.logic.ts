import type { AssistantState } from "@assistant-ui/react";

import { isAgentEventPayload, parseToolUseResult } from "../lib/message-parts";
import type { CostDisplay } from "@/lib/cost-provenance";

export type ThreadMessageState = AssistantState["thread"]["messages"][number];

export type InspectorTraceKind =
  | "start"
  | "user"
  | "agent"
  | "tool"
  | "diff"
  | "system"
  | "approval";

export type InspectorTraceStatus = "ok" | "warn" | "error" | "pending";

export interface InspectorTraceEvent {
  id: string;
  kind: InspectorTraceKind;
  label: string;
  timestamp: number;
  status: InspectorTraceStatus;
}

export interface InspectorFileEntry {
  path: string;
  readCount: number;
}

export const TRACE_LIMIT_DEFAULT = 6;

function traceKindFromRole(role: ThreadMessageState["role"]): InspectorTraceKind {
  switch (role) {
    case "user":
      return "user";
    case "assistant":
      return "agent";
    case "system":
      return "system";
  }

  const _exhaustive: never = role;
  return _exhaustive;
}

function traceStatusFromMessage(message: ThreadMessageState): InspectorTraceStatus {
  if (message.role !== "assistant") {
    return "ok";
  }

  if (message.status?.type === "running" || message.status?.type === "requires-action") {
    return "pending";
  }

  if (message.status?.type === "incomplete") {
    return message.status.reason === "error" ? "error" : "warn";
  }

  return "ok";
}

function toolStatusFromPart(part: ThreadMessageState["parts"][number]): InspectorTraceStatus {
  if (part.type !== "tool-call") {
    return "ok";
  }

  if (part.isError || (part.status.type === "incomplete" && part.status.reason === "error")) {
    return "error";
  }

  if (part.status.type === "running" || part.status.type === "requires-action") {
    return "pending";
  }

  if (part.status.type === "incomplete") {
    return "warn";
  }

  return "ok";
}

function getTextPartText(message: ThreadMessageState): string {
  return message.content.reduce((text, part) => {
    if (part.type !== "text" && part.type !== "reasoning") {
      return text;
    }
    return `${text}${part.text}`;
  }, "");
}

function traceLabelFromMessage(message: ThreadMessageState): string {
  if (message.role === "system") {
    const first = getTextPartText(message).split("\n")[0] ?? "";
    return first || "system event";
  }

  if (message.role === "user") {
    return "Prompt sent";
  }

  return "Agent response";
}

/** Map the current thread messages into the latest Inspector trace rows. */
export function deriveTraceEvents(
  messages: readonly ThreadMessageState[],
  limit = TRACE_LIMIT_DEFAULT
): InspectorTraceEvent[] {
  if (messages.length === 0) {
    return [];
  }

  const events: InspectorTraceEvent[] = [];
  const firstTimestamp = messages[0]?.createdAt.getTime() ?? Date.now();
  events.push({
    id: `start-${messages[0]?.id ?? "session"}`,
    kind: "start",
    label: "Session started",
    timestamp: firstTimestamp,
    status: "ok",
  });

  for (const message of messages) {
    const timestamp = message.createdAt.getTime();

    if (message.role === "user" || message.role === "system") {
      events.push({
        id: message.id,
        kind: traceKindFromRole(message.role),
        label: traceLabelFromMessage(message),
        timestamp,
        status: traceStatusFromMessage(message),
      });
      continue;
    }

    const hasAssistantNarration = message.content.some(
      part => part.type === "text" || part.type === "reasoning"
    );

    if (hasAssistantNarration) {
      events.push({
        id: message.id,
        kind: "agent",
        label: traceLabelFromMessage(message),
        timestamp,
        status: traceStatusFromMessage(message),
      });
    }

    for (const part of message.parts) {
      if (part.type === "tool-call") {
        events.push({
          id: part.toolCallId,
          kind: "tool",
          label: part.toolName || "tool call",
          timestamp,
          status: toolStatusFromPart(part),
        });
      }

      if (part.type === "data" && part.name === "agh-permission") {
        const raw = part.data as { title?: string; decision?: string } | undefined;
        events.push({
          id: `${message.id}-${part.name}`,
          kind: "approval",
          label: raw?.title || "Permission required",
          timestamp,
          status: raw?.decision ? "ok" : "pending",
        });
      }
    }
  }

  return events.slice(-limit);
}

/** Aggregate tool messages by file path while preserving first-seen order. */
export function deriveFileReads(messages: readonly ThreadMessageState[]): InspectorFileEntry[] {
  const index = new Map<string, InspectorFileEntry>();
  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }

    for (const part of message.parts) {
      if (part.type !== "tool-call") {
        continue;
      }

      const result = isAgentEventPayload(part.result) ? parseToolUseResult(part.result) : null;
      const path = result?.filePath ?? readFilePathFromInput(part.args);
      if (!path) {
        continue;
      }

      const existing = index.get(path);
      if (existing) {
        existing.readCount += 1;
      } else {
        index.set(path, { path, readCount: 1 });
      }
    }
  }
  return Array.from(index.values());
}

function readFilePathFromInput(input: Record<string, unknown> | undefined): string | undefined {
  if (!input) {
    return undefined;
  }
  const raw = input.file_path ?? input.filePath ?? input.path;
  return typeof raw === "string" && raw.length > 0 ? raw : undefined;
}

/** Token/turn fields the Usage-presence predicate reads (structural subset of InspectorUsage). */
interface UsagePresenceFields {
  tokensIn?: number;
  tokensOut?: number;
  totalTokens?: number;
  turnCount?: number;
}

/**
 * The Usage panel opens for any reported token counter, an authoritative cost
 * status (`cost.hasCost`, owned by describeCost), or a positive turn count. A
 * statusless amount alone never opens it: describeCost reports `hasCost: false`
 * for it, so this predicate excludes it by construction.
 */
export function hasReportableUsage(usage: UsagePresenceFields, cost: CostDisplay): boolean {
  return (
    usage.tokensIn !== undefined ||
    usage.tokensOut !== undefined ||
    usage.totalTokens !== undefined ||
    cost.hasCost ||
    (usage.turnCount ?? 0) > 0
  );
}
