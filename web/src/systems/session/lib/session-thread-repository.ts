import {
  ExportedMessageRepository,
  type ThreadMessage,
  type ThreadMessageLike,
} from "@assistant-ui/react";

import type { SessionMessage } from "../types";
import { toolPartResult } from "./tool-part-error";

type SessionMessagePart = NonNullable<SessionMessage["parts"]>[number];
type ThreadContentPart = Exclude<ThreadMessageLike["content"], string>[number];
type SessionMessageWithStatus = SessionMessage & { status?: ThreadMessageLike["status"] };
type ExportedThreadMessageItem = { message: ThreadMessage };
type JSONValue = null | string | number | boolean | readonly JSONValue[] | JSONObject;
type JSONObject = { readonly [key: string]: JSONValue };

const threadMessageBySource = new WeakMap<SessionMessage, ThreadMessage>();

interface ThreadMessageSequenceCacheNode {
  children: WeakMap<SessionMessage, ThreadMessageSequenceCacheNode>;
  result?: ThreadMessage[];
}

const emptyThreadMessages: ThreadMessage[] = [];
const threadMessageSequenceRoot: ThreadMessageSequenceCacheNode = {
  children: new WeakMap(),
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" ? value : undefined;
}

function jsonText(value: unknown): string {
  if (value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function isJSONValue(value: unknown, depth: number = 0): value is JSONValue {
  if (depth > 100) {
    return false;
  }

  if (value === null || typeof value === "string" || typeof value === "boolean") {
    return true;
  }

  if (typeof value === "number") {
    return Number.isFinite(value);
  }

  if (Array.isArray(value)) {
    return value.every(item => isJSONValue(item, depth + 1));
  }

  if (isRecord(value)) {
    return Object.values(value).every(item => isJSONValue(item, depth + 1));
  }

  return false;
}

function toJSONObject(value: unknown): JSONObject {
  if (!isRecord(value)) {
    return {};
  }
  return isJSONValue(value) ? value : {};
}

// `partIndex` is the part's position in the daemon's projected `message.parts`:
// the identity search results (`part_index`) name, carried through the runtime
// so a row can tell it holds the matched part.
function threadPartMetadata(
  record: Record<string, unknown>,
  partIndex?: number
): Record<string, string> {
  const metadata: Record<string, string> = {};
  if (partIndex !== undefined) {
    metadata.partIndex = String(partIndex);
  }
  const turnId = stringField(record, "turn_id") ?? stringField(record, "turnId");
  if (turnId) {
    metadata.turnId = turnId;
  }
  const timestamp = stringField(record, "timestamp");
  if (timestamp) {
    metadata.timestamp = timestamp;
  }
  const state = stringField(record, "state");
  if (state) {
    metadata.state = state;
  }
  return metadata;
}

function toToolPart(
  record: Record<string, unknown>,
  type: string,
  partIndex?: number
): ThreadContentPart {
  const toolName = type.slice("tool-".length).trim() || stringField(record, "toolName") || "tool";
  const toolCallId =
    stringField(record, "toolCallId") || stringField(record, "tool_call_id") || `${toolName}-call`;
  const input = record.input;
  const state = stringField(record, "state");
  const isError = state === "output-error" || Boolean(record.isError);

  return {
    type: "tool-call" as const,
    toolCallId,
    toolName,
    args: toJSONObject(input),
    argsText: jsonText(input),
    result: toolPartResult(record, isError),
    isError,
    ...threadPartMetadata(record, partIndex),
  } as ThreadContentPart;
}

const TRANSCRIPT_MARKER_EVENT_PREFIX = "transcript_marker.";

/**
 * The daemon's persisted marker events carry the marker under `raw`
 * (`{kind, occurred_at, summary, evidence}`) while live-streamed ones may carry
 * `marker`; every consumer reads `marker`, so the boundary sets it once from
 * `raw` and keeps `raw` untouched.
 */
function normalizeCompozyEvent(data: unknown): unknown {
  if (!isRecord(data) || isRecord(data.marker)) return data;
  const type = stringField(data, "type") ?? "";
  const raw = data.raw;
  if (!type.startsWith(TRANSCRIPT_MARKER_EVENT_PREFIX) || !isRecord(raw)) return data;
  const kind = stringField(raw, "kind");
  if (!kind) return data;
  return {
    ...data,
    marker: {
      kind,
      summary: stringField(raw, "summary") ?? stringField(data, "text") ?? "",
      occurred_at: stringField(raw, "occurred_at") ?? stringField(data, "timestamp") ?? "",
      ...(isRecord(raw.evidence) ? { evidence: raw.evidence } : {}),
      ...(raw.diagnostic !== undefined ? { diagnostic: raw.diagnostic } : {}),
    },
  };
}

interface MessageBoundaryFacts {
  turnId?: string;
  /** Earliest `tool_call` event time per tool call id, from the message's own events. */
  toolCallTimes: ReadonlyMap<string, string>;
}

// Tool parts carry no time of their own; the daemon's `tool_call` events in the
// same message do, keyed by the call id. Text and tool parts also carry no
// turn id; the message's turn (from its events or its metadata) is theirs.
function messageBoundaryFacts(message: SessionMessage): MessageBoundaryFacts {
  const toolCallTimes = new Map<string, string>();
  let turnId: string | undefined;
  for (const part of message.parts ?? []) {
    if (!isRecord(part)) continue;
    const type = stringField(part, "type") ?? "";
    if (!type.startsWith("data-")) continue;
    const data = (part as { data?: unknown }).data;
    if (!isRecord(data)) continue;
    turnId ??= stringField(data, "turn_id");
    const toolCallId = stringField(data, "tool_call_id");
    const timestamp = stringField(data, "timestamp");
    if (data.type === "tool_call" && toolCallId && timestamp && !toolCallTimes.has(toolCallId)) {
      toolCallTimes.set(toolCallId, timestamp);
    }
  }
  return {
    turnId:
      turnId ?? (isRecord(message.metadata) ? stringField(message.metadata, "turn_id") : undefined),
    toolCallTimes,
  };
}

function withBoundaryFacts(
  part: Record<string, unknown>,
  type: string,
  facts: MessageBoundaryFacts
): Record<string, unknown> {
  let next = part;
  if (facts.turnId && !stringField(part, "turn_id") && !stringField(part, "turnId")) {
    next = { ...next, turnId: facts.turnId };
  }
  if (type.startsWith("tool-") && !stringField(part, "timestamp")) {
    const timestamp = facts.toolCallTimes.get(stringField(part, "toolCallId") ?? "");
    if (timestamp) next = { ...next, timestamp };
  }
  return next;
}

const NO_BOUNDARY_FACTS: MessageBoundaryFacts = { toolCallTimes: new Map() };

function toThreadPart(
  sourcePart: SessionMessagePart,
  partIndex?: number,
  facts: MessageBoundaryFacts = NO_BOUNDARY_FACTS
): ThreadContentPart | null {
  if (!isRecord(sourcePart)) {
    return null;
  }

  const type = stringField(sourcePart, "type");
  if (!type) {
    return null;
  }
  const part = withBoundaryFacts(sourcePart, type, facts);

  if (type === "text") {
    return {
      type: "text" as const,
      text: stringField(part, "text") ?? "",
      ...threadPartMetadata(part, partIndex),
    } as ThreadContentPart;
  }

  if (type === "reasoning") {
    return {
      type: "reasoning" as const,
      text: stringField(part, "text") ?? "",
      ...threadPartMetadata(part, partIndex),
    } as ThreadContentPart;
  }

  if (type.startsWith("data-")) {
    const data = (part as { data?: unknown }).data;
    return {
      type: type as `data-${string}`,
      data: type === "data-compozy-event" ? normalizeCompozyEvent(data) : data,
      ...threadPartMetadata(part, partIndex),
    } as ThreadContentPart;
  }

  if (type.startsWith("tool-")) {
    return toToolPart(part, type, partIndex);
  }

  if (type === "file") {
    const url = stringField(part, "url") ?? stringField(part, "data");
    const mimeType = stringField(part, "mediaType") ?? stringField(part, "mimeType");
    const filename = stringField(part, "filename");
    if (!url || !mimeType) {
      return null;
    }
    return {
      type: "file" as const,
      data: url,
      mimeType,
      ...(filename ? { filename } : {}),
      ...threadPartMetadata(part),
    } as ThreadContentPart;
  }

  return null;
}

function toThreadRole(role: SessionMessage["role"]): ThreadMessageLike["role"] {
  if (role === "user" || role === "assistant" || role === "system") {
    return role;
  }
  return "assistant";
}

function toThreadMetadata(message: SessionMessage): ThreadMessageLike["metadata"] | undefined {
  if (!isRecord(message.metadata)) {
    return undefined;
  }
  return { custom: message.metadata };
}

export function toThreadMessageLikes(messages: SessionMessage[]): ThreadMessageLike[] {
  return messages.map(message => {
    const facts = messageBoundaryFacts(message);
    const parts =
      message.parts
        ?.map((part, index) => toThreadPart(part, index, facts))
        .filter(part => part !== null) ?? [];
    const role = toThreadRole(message.role);
    const status = role === "assistant" ? (message as SessionMessageWithStatus).status : undefined;
    const metadata = toThreadMetadata(message);
    return {
      id: message.id,
      role,
      content: parts,
      status,
      metadata,
    } satisfies ThreadMessageLike;
  });
}

function toReadonlyThreadMessage(message: SessionMessage): ThreadMessage {
  const cached = threadMessageBySource.get(message);
  if (cached) return cached;

  const repository: { messages: ExportedThreadMessageItem[] } = ExportedMessageRepository.fromArray(
    toThreadMessageLikes([message])
  );
  const converted = repository.messages[0]?.message;
  if (!converted) {
    throw new Error(`Failed to convert transcript message "${message.id}"`);
  }
  threadMessageBySource.set(message, converted);
  return converted;
}

export function toReadonlyThreadMessages(messages: SessionMessage[]): ThreadMessage[] {
  if (messages.length === 0) return emptyThreadMessages;

  let node = threadMessageSequenceRoot;
  for (const message of messages) {
    let child = node.children.get(message);
    if (!child) {
      child = { children: new WeakMap() };
      node.children.set(message, child);
    }
    node = child;
  }

  node.result ??= messages.map(toReadonlyThreadMessage);
  return node.result;
}
