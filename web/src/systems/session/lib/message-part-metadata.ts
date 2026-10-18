import type { SessionMessage } from "../types";

type SessionMessagePart = NonNullable<SessionMessage["parts"]>[number];

interface PartMetadata {
  turnId?: string;
  /** Provider-native subagent attribution (S8); text parts would otherwise join the parent flow. */
  parentToolCallId?: string;
  timestamp?: string;
  bytes?: number;
  width?: number;
  height?: number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function numberField(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

// `validateUIMessages` parses every part against the AI SDK's schema, which drops
// unknown sibling keys — so CompozyOS's custom metadata fields never survive
// validation. Turn folding and attachment rendering need their turn, time, and
// attachment metadata, so capture it before validation and re-attach afterward. Validation
// preserves part order/count (an invalid part fails the whole message and throws),
// so index alignment is sound; the length guard is purely defensive.
export function capturePartMetadata(messages: unknown): PartMetadata[][] {
  if (!Array.isArray(messages)) {
    return [];
  }
  return messages.map(message => {
    if (!isRecord(message) || !Array.isArray(message.parts)) {
      return [];
    }
    return message.parts.map((part): PartMetadata => {
      if (!isRecord(part)) {
        return {};
      }
      const metadata: PartMetadata = {};
      const turnId = stringField(part, "turnId") ?? stringField(part, "turn_id");
      if (turnId) {
        metadata.turnId = turnId;
      }
      const timestamp = stringField(part, "timestamp");
      if (timestamp) {
        metadata.timestamp = timestamp;
      }
      const parentToolCallId = stringField(part, "parentToolCallId");
      if (parentToolCallId) {
        metadata.parentToolCallId = parentToolCallId;
      }
      const bytes = numberField(part, "bytes");
      if (bytes !== undefined) {
        metadata.bytes = bytes;
      }
      const width = numberField(part, "width");
      if (width !== undefined) {
        metadata.width = width;
      }
      const height = numberField(part, "height");
      if (height !== undefined) {
        metadata.height = height;
      }
      return metadata;
    });
  });
}

export function reattachPartMetadata(
  validated: SessionMessage[],
  captured: PartMetadata[][]
): SessionMessage[] {
  return validated.map((message, index) => {
    const parts = message.parts;
    const metas = captured[index];
    if (!metas || !Array.isArray(parts) || parts.length !== metas.length) {
      return message;
    }
    let mutated = false;
    const nextParts = parts.map((part, partIndex) => {
      const meta = metas[partIndex];
      if (
        !meta ||
        (!meta.turnId &&
          !meta.timestamp &&
          !meta.parentToolCallId &&
          meta.bytes === undefined &&
          meta.width === undefined &&
          meta.height === undefined)
      ) {
        return part;
      }
      mutated = true;
      return {
        ...part,
        ...(meta.turnId ? { turnId: meta.turnId } : {}),
        ...(meta.timestamp ? { timestamp: meta.timestamp } : {}),
        ...(meta.parentToolCallId ? { parentToolCallId: meta.parentToolCallId } : {}),
        ...(meta.bytes !== undefined ? { bytes: meta.bytes } : {}),
        ...(meta.width !== undefined ? { width: meta.width } : {}),
        ...(meta.height !== undefined ? { height: meta.height } : {}),
      } as SessionMessagePart;
    });
    return mutated ? ({ ...message, parts: nextParts } as SessionMessage) : message;
  });
}
