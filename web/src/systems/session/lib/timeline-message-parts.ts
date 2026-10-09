// Shared converter from an assistant-ui message to the pure `SessionTimelinePart[]`
// the derive layer consumes. Extracted so both the render hook
// (`use-assistant-message-timeline.ts`) and the virtualizer's per-row-kind size
// estimator (`timeline-row-estimates.ts`) read the message the same way — one
// source of truth for how the runtime's part shapes map onto the row model.

import { isInterruptedState, type SessionTimelinePart } from "./session-timeline-parts";

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
  // A subagent card names its spawning turn itself; it never borrows the
  // message's, which could place it in the wrong turn's group or fold.
  return isSubagentCardPart(part) ? undefined : fallbackTurnId;
}

function isSubagentCardPart(part: Record<string, unknown>): boolean {
  const type = stringField(part, "type");
  return (
    type === "data-compozy-subagent" ||
    (type === "data" && stringField(part, "name") === "compozy-subagent")
  );
}

// The daemon's projected part position (search results name it as `part_index`).
function partIndexOf(part: Record<string, unknown>): number | undefined {
  const raw = stringField(part, "partIndex");
  if (raw === undefined) return undefined;
  const index = Number.parseInt(raw, 10);
  return Number.isInteger(index) && index >= 0 ? index : undefined;
}

function partTimestamp(part: Record<string, unknown>): string | undefined {
  const own = stringField(part, "timestamp");
  if (own) return own;
  const data = part.data;
  return isRecord(data) ? stringField(data, "timestamp") : undefined;
}

// Provider-native attribution (S8): the transcript projection names it on the
// part (`parentToolCallId`); the live prompt stream carries it in the AI SDK
// provider metadata (`providerMetadata.compozy.parentToolCallId`).
function partParentToolCallId(part: Record<string, unknown>): string | undefined {
  const own = stringField(part, "parentToolCallId");
  if (own) return own;
  const metadata = part.providerMetadata;
  if (!isRecord(metadata) || !isRecord(metadata.compozy)) return undefined;
  return stringField(metadata.compozy, "parentToolCallId") || undefined;
}

/** Projects thread parts without discarding provider titles or original tool inputs. */
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
    const partIndex = partIndexOf(part);
    const parentToolCallId = partParentToolCallId(part);
    const attribution = parentToolCallId ? { parentToolCallId } : {};
    const type = stringField(part, "type");
    if (type === "text") {
      return [
        {
          kind: "text",
          id,
          text: stringField(part, "text") ?? "",
          turnId,
          timestamp,
          state,
          partIndex,
          ...attribution,
        },
      ];
    }
    if (type === "reasoning") {
      return [
        {
          kind: "reasoning",
          id,
          text: stringField(part, "text") ?? "",
          turnId,
          timestamp,
          state,
          partIndex,
          ...attribution,
        },
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
          toolTitle: stringField(part, "toolTitle"),
          args: recordField(part, "args"),
          result,
          isError,
          status: isInterruptedState(state)
            ? "interrupted"
            : result === undefined && !isError
              ? "running"
              : "settled",
          turnId,
          timestamp,
          state,
          partIndex,
          ...attribution,
        },
      ];
    }
    if (type === "data" || (typeof type === "string" && type.startsWith("data-"))) {
      const dataName = type === "data" ? stringField(part, "name") : type.slice("data-".length);
      const name = dataName ? `data-${dataName}` : "data";
      return [{ kind: "data", id, name, data: part.data, turnId, timestamp, state, partIndex }];
    }
    return [];
  });
}
