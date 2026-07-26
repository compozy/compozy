// Shared converter from an assistant-ui message to the pure `SessionTimelinePart[]`
// the derive layer consumes. Extracted so both the render hook
// (`use-assistant-message-timeline.ts`) and the virtualizer's per-row-kind size
// estimator (`timeline-row-estimates.ts`) read the message the same way — one
// source of truth for how the runtime's part shapes map onto the row model.

import type { SessionTimelinePart } from "./session-timeline.logic";

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" ? value : undefined;
}

function recordField(record: Record<string, unknown>, key: string): Record<string, unknown> {
  const value = record[key];
  return isRecord(value) ? value : {};
}

// Each assistant message is one turn (`message.id` is the turn id), so parts that
// carry no explicit turn id still belong to the message's turn. Data-event parts
// carry the turn id and the only per-part timestamps the runtime emits inside
// their payload, so surface both for turn grouping and the "Worked for Xs"
// duration when text/tool parts have none.
function partTurnId(
  part: Record<string, unknown>,
  fallbackTurnId: string | undefined
): string | undefined {
  const own = stringField(part, "turnId") ?? stringField(part, "turn_id");
  if (own) return own;
  const data = part.data;
  if (isRecord(data)) {
    const fromData = stringField(data, "turn_id") ?? stringField(data, "turnId");
    if (fromData) return fromData;
  }
  return fallbackTurnId;
}

function partTimestamp(part: Record<string, unknown>): string | undefined {
  const own = stringField(part, "timestamp");
  if (own) return own;
  const data = part.data;
  return isRecord(data) ? stringField(data, "timestamp") : undefined;
}

export function toTimelineParts(message: {
  id?: string;
  content?: unknown;
}): SessionTimelinePart[] {
  const content = Array.isArray(message.content) ? message.content : [];
  const fallbackTurnId =
    typeof message.id === "string" && message.id.length > 0 ? message.id : undefined;
  return content.flatMap((part, index): SessionTimelinePart[] => {
    if (!isRecord(part)) return [];
    const id =
      stringField(part, "id") ??
      stringField(part, "toolCallId") ??
      `${message.id ?? "message"}:${index}`;
    const turnId = partTurnId(part, fallbackTurnId);
    const timestamp = partTimestamp(part);
    const state = stringField(part, "state");
    const type = stringField(part, "type");
    if (type === "text") {
      return [
        { kind: "text", id, text: stringField(part, "text") ?? "", turnId, timestamp, state },
      ];
    }
    if (type === "reasoning") {
      return [
        { kind: "reasoning", id, text: stringField(part, "text") ?? "", turnId, timestamp, state },
      ];
    }
    if (type === "tool-call") {
      const result = part.result;
      const isError = part.isError === true;
      return [
        {
          kind: "tool",
          id,
          toolCallId: stringField(part, "toolCallId") ?? id,
          toolName: stringField(part, "toolName") ?? "tool",
          args: recordField(part, "args"),
          result,
          isError,
          status: result === undefined && !isError ? "running" : "settled",
          turnId,
          timestamp,
          state,
        },
      ];
    }
    if (type === "data" || (typeof type === "string" && type.startsWith("data-"))) {
      const dataName = type === "data" ? stringField(part, "name") : type.slice("data-".length);
      const name = dataName ? `data-${dataName}` : "data";
      return [{ kind: "data", id, name, data: part.data, turnId, timestamp, state }];
    }
    return [];
  });
}
