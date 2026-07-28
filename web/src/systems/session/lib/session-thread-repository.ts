import {
  ExportedMessageRepository,
  type ThreadMessage,
  type ThreadMessageLike,
} from "@assistant-ui/react";

import type { SessionMessage } from "../types";

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

function threadPartMetadata(record: Record<string, unknown>): Record<string, string> {
  const metadata: Record<string, string> = {};
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

function toToolPart(record: Record<string, unknown>, type: string): ThreadContentPart {
  const toolName = type.slice("tool-".length).trim() || stringField(record, "toolName") || "tool";
  const toolCallId =
    stringField(record, "toolCallId") || stringField(record, "tool_call_id") || `${toolName}-call`;
  const input = record.input;
  const state = stringField(record, "state");

  return {
    type: "tool-call" as const,
    toolCallId,
    toolName,
    args: toJSONObject(input),
    argsText: jsonText(input),
    result: record.output,
    isError: state === "output-error" || Boolean(record.isError),
    ...threadPartMetadata(record),
  } as ThreadContentPart;
}

function toThreadPart(part: SessionMessagePart): ThreadContentPart | null {
  if (!isRecord(part)) {
    return null;
  }

  const type = stringField(part, "type");
  if (!type) {
    return null;
  }

  if (type === "text") {
    return {
      type: "text" as const,
      text: stringField(part, "text") ?? "",
      ...threadPartMetadata(part),
    } as ThreadContentPart;
  }

  if (type === "reasoning") {
    return {
      type: "reasoning" as const,
      text: stringField(part, "text") ?? "",
      ...threadPartMetadata(part),
    } as ThreadContentPart;
  }

  if (type.startsWith("data-")) {
    return {
      type: type as `data-${string}`,
      data: (part as { data?: unknown }).data,
      ...threadPartMetadata(part),
    } as ThreadContentPart;
  }

  if (type.startsWith("tool-")) {
    return toToolPart(part, type);
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
    const parts = message.parts?.map(toThreadPart).filter(part => part !== null) ?? [];
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
