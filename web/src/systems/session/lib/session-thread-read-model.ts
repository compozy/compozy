import type { ThreadMessage } from "@assistant-ui/react";

type MessageSource = "transcript" | "runtime";

const TURN_ID_KEYS = ["turn_id", "turnId"] as const;
const CLIENT_TEMP_ID_KEYS = [
  "client_temp_id",
  "clientTempId",
  "client_message_id",
  "clientMessageId",
  "temp_id",
  "tempId",
] as const;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown> | undefined, keys: readonly string[]) {
  if (!record) {
    return null;
  }
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value.trim().length > 0) {
      return value;
    }
  }
  return null;
}

function messageCustom(message: ThreadMessage): Record<string, unknown> | undefined {
  const custom = message.metadata?.custom;
  return isRecord(custom) ? custom : undefined;
}

function dataRecords(message: ThreadMessage): Record<string, unknown>[] {
  const records: Record<string, unknown>[] = [];
  for (const part of message.content) {
    if (!isRecord(part)) {
      continue;
    }
    if ("data" in part && isRecord(part.data)) {
      records.push(part.data);
    }
  }
  return records;
}

function promotionKeys(message: ThreadMessage, source: MessageSource): string[] {
  const keys = new Set<string>();
  const candidates = [messageCustom(message), ...dataRecords(message)];

  for (const candidate of candidates) {
    const turnId = stringField(candidate, TURN_ID_KEYS);
    const clientTempId = stringField(candidate, CLIENT_TEMP_ID_KEYS);
    if (clientTempId) {
      keys.add(`client:${clientTempId}`);
    }
    if (turnId && clientTempId) {
      keys.add(`${turnId}:${clientTempId}`);
    }
    if (turnId && source === "runtime") {
      keys.add(`${turnId}:${message.id}`);
    }
  }
  if (source === "runtime" && message.role === "user") {
    keys.add(`client:${message.id}`);
  }

  return [...keys];
}

function transcriptReconciliationIndex(transcriptMessages: readonly ThreadMessage[]) {
  return {
    ids: new Set(transcriptMessages.map(message => message.id)),
    promotionKeys: new Set(
      transcriptMessages.flatMap(message => promotionKeys(message, "transcript"))
    ),
  };
}

function isRuntimeMessageReconciled(
  message: ThreadMessage,
  transcriptIndex: ReturnType<typeof transcriptReconciliationIndex>
): boolean {
  if (transcriptIndex.ids.has(message.id)) return true;
  return promotionKeys(message, "runtime").some(key => transcriptIndex.promotionKeys.has(key));
}

export function hasUnreconciledRuntimeMessages({
  transcriptMessages,
  runtimeMessages,
}: {
  transcriptMessages: readonly ThreadMessage[];
  runtimeMessages: readonly ThreadMessage[];
}): boolean {
  const transcriptIndex = transcriptReconciliationIndex(transcriptMessages);
  return runtimeMessages.some(message => !isRuntimeMessageReconciled(message, transcriptIndex));
}

export function mergeSessionThreadReadModel({
  transcriptMessages,
  runtimeMessages,
  includeRuntimeTail = true,
}: {
  transcriptMessages: readonly ThreadMessage[];
  runtimeMessages: readonly ThreadMessage[];
  includeRuntimeTail?: boolean;
}): readonly ThreadMessage[] {
  if (runtimeMessages.length === 0 || !includeRuntimeTail) {
    return transcriptMessages;
  }

  const transcriptIndex = transcriptReconciliationIndex(transcriptMessages);
  const optimisticTail: ThreadMessage[] = [];

  for (const message of runtimeMessages) {
    if (isRuntimeMessageReconciled(message, transcriptIndex)) continue;
    optimisticTail.push(message);
  }

  return optimisticTail.length === 0
    ? transcriptMessages
    : [...transcriptMessages, ...optimisticTail];
}
