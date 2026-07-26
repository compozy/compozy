import { validateUIMessages } from "ai";
import { z } from "zod";

import type { SessionMessage } from "../types";

const aghEventDataSchema = z.looseObject({
  type: z.string(),
  session_id: z.string().optional(),
  turn_id: z.string().optional(),
  request_id: z.string().optional(),
  timestamp: z.string().optional(),
  text: z.string().optional(),
  title: z.string().optional(),
  tool_call_id: z.string().optional(),
  stop_reason: z.string().optional(),
  action: z.string().optional(),
  resource: z.string().optional(),
  decision: z.string().optional(),
  error: z.string().optional(),
  usage: z
    .object({
      turn_id: z.string().optional(),
      input_tokens: z.number().optional(),
      output_tokens: z.number().optional(),
      total_tokens: z.number().optional(),
      thought_tokens: z.number().optional(),
      cache_read_tokens: z.number().optional(),
      cache_write_tokens: z.number().optional(),
      context_used: z.number().optional(),
      context_size: z.number().optional(),
      cost_amount: z.number().optional(),
      cost_currency: z.string().optional(),
      timestamp: z.string().optional(),
    })
    .optional(),
  runtime: z
    .object({
      turn_id: z.string().optional(),
      turn_source: z.string().optional(),
      turn_started_at: z.string().nullable().optional(),
      deadline_at: z.string().nullable().optional(),
      last_activity_at: z.string().nullable().optional(),
      last_activity_kind: z.string().optional(),
      last_activity_detail: z.string().optional(),
      current_tool: z.string().optional(),
      tool_call_id: z.string().optional(),
      last_progress_at: z.string().nullable().optional(),
      iteration_current: z.number().optional(),
      iteration_max: z.number().optional(),
      idle_seconds: z.number().optional(),
      elapsed_ms: z.number(),
      elapsed_seconds: z.number().optional(),
    })
    .optional(),
  goal: z
    .object({
      kind: z.enum(["goal-work", "goal-continuation", "goal-compaction"]),
      run_id: z.string(),
      node_id: z.string(),
      generation: z.number(),
      item_index: z.number(),
      turn: z.number().nullable(),
      prompt_attempt: z.number(),
      prompt_id: z.string(),
    })
    .optional(),
  raw: z.unknown().optional(),
});

const aghPermissionDataSchema = aghEventDataSchema.extend({
  request_id: z.string(),
  raw: z.record(z.string(), z.unknown()).optional(),
});

const unknownDataSchema = z.unknown();

const knownDataSchemas: Record<string, z.ZodType<unknown>> = {
  "agh-event": aghEventDataSchema,
  "agh-permission": aghPermissionDataSchema,
};

type SessionMessagePart = NonNullable<SessionMessage["parts"]>[number];

interface PartTurnMeta {
  turnId?: string;
  timestamp?: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function isToolPart(part: unknown): part is Record<string, unknown> {
  return isRecord(part) && typeof part.type === "string" && part.type.startsWith("tool-");
}

function toolNameForValidation(part: Record<string, unknown>, type: string): string {
  const toolName = stringField(part, "toolName") ?? type.slice("tool-".length).trim();
  return toolName || "tool";
}

function toolCallIdForValidation(part: Record<string, unknown>, toolName: string): string {
  return (
    stringField(part, "toolCallId") ??
    stringField(part, "tool_call_id") ??
    stringField(part, "id") ??
    `${toolName}-call`
  );
}

function outputErrorText(part: Record<string, unknown>): string {
  const direct = stringField(part, "errorText") ?? stringField(part, "error");
  if (direct) {
    return direct;
  }

  const output = part.output;
  if (isRecord(output)) {
    const fromOutput =
      stringField(output, "error") ?? stringField(output, "text") ?? stringField(output, "title");
    if (fromOutput) {
      return fromOutput;
    }
  }

  return "Tool execution failed";
}

function normalizeToolPartForValidation(part: unknown): unknown {
  if (!isToolPart(part)) {
    return part;
  }

  const type = stringField(part, "type");
  if (!type) {
    return part;
  }

  const toolName = toolNameForValidation(part, type);
  const state = stringField(part, "state");
  const validationPart: Record<string, unknown> = {
    ...part,
    type: "dynamic-tool",
    toolName,
    toolCallId: toolCallIdForValidation(part, toolName),
  };

  delete validationPart.tool_call_id;

  switch (state) {
    case "input-streaming":
    case "input-available":
    case "approval-requested":
    case "approval-responded":
    case "output-denied":
      delete validationPart.output;
      delete validationPart.errorText;
      return validationPart;
    case "output-available":
      delete validationPart.errorText;
      return validationPart;
    case "output-error":
      validationPart.errorText = outputErrorText(part);
      delete validationPart.output;
      return validationPart;
    default:
      return validationPart;
  }
}

function messagesForValidation(messages: unknown): unknown {
  if (!Array.isArray(messages)) {
    return messages;
  }

  let mutated = false;
  const normalizedMessages = messages.map(message => {
    if (!isRecord(message) || !Array.isArray(message.parts)) {
      return message;
    }

    let messageMutated = false;
    const normalizedParts = message.parts.map(part => {
      const normalized = normalizeToolPartForValidation(part);
      if (normalized !== part) {
        messageMutated = true;
      }
      return normalized;
    });
    if (!messageMutated) {
      return message;
    }
    mutated = true;
    return { ...message, parts: normalizedParts };
  });

  return mutated ? normalizedMessages : messages;
}

function restoreOriginalToolParts(
  validated: SessionMessage[],
  originalMessages: unknown
): SessionMessage[] {
  if (!Array.isArray(originalMessages)) {
    return validated;
  }

  return validated.map((message, messageIndex) => {
    const originalMessage = originalMessages[messageIndex];
    if (!isRecord(originalMessage) || !Array.isArray(originalMessage.parts)) {
      return message;
    }
    const originalParts = originalMessage.parts;
    const parts = message.parts;
    if (!Array.isArray(parts) || parts.length !== originalParts.length) {
      return message;
    }

    let mutated = false;
    const restoredParts = parts.map((part, partIndex) => {
      const originalPart = originalParts[partIndex];
      if (!isToolPart(originalPart)) {
        return part;
      }
      mutated = true;
      return originalPart as SessionMessagePart;
    });

    return mutated ? ({ ...message, parts: restoredParts } as SessionMessage) : message;
  });
}

// `validateUIMessages` parses every part against the AI SDK's schema, which drops
// unknown sibling keys — so AGH's custom `turn_id`/`timestamp` fields never survive
// validation. The turn-fold derivation needs them (turn boundaries + "Worked for Xs"
// duration), so capture them before validation and re-attach afterward. Validation
// preserves part order/count (an invalid part fails the whole message and throws),
// so index alignment is sound; the length guard is purely defensive.
function capturePartTurnMeta(messages: unknown): PartTurnMeta[][] {
  if (!Array.isArray(messages)) {
    return [];
  }
  return messages.map(message => {
    if (!isRecord(message) || !Array.isArray(message.parts)) {
      return [];
    }
    return message.parts.map((part): PartTurnMeta => {
      if (!isRecord(part)) {
        return {};
      }
      const meta: PartTurnMeta = {};
      const turnId = stringField(part, "turnId") ?? stringField(part, "turn_id");
      if (turnId) {
        meta.turnId = turnId;
      }
      const timestamp = stringField(part, "timestamp");
      if (timestamp) {
        meta.timestamp = timestamp;
      }
      return meta;
    });
  });
}

function reattachPartTurnMeta(
  validated: SessionMessage[],
  captured: PartTurnMeta[][]
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
      if (!meta || (!meta.turnId && !meta.timestamp)) {
        return part;
      }
      mutated = true;
      return {
        ...part,
        ...(meta.turnId ? { turnId: meta.turnId } : {}),
        ...(meta.timestamp ? { timestamp: meta.timestamp } : {}),
      } as SessionMessagePart;
    });
    return mutated ? ({ ...message, parts: nextParts } as SessionMessage) : message;
  });
}

function dataPartName(part: unknown): string | null {
  if (!isRecord(part) || typeof part.type !== "string" || !part.type.startsWith("data-")) {
    return null;
  }

  const name = part.type.slice(5);
  return name === "" ? null : name;
}

function dataSchemasForMessages(messages: unknown): Record<string, z.ZodType<unknown>> {
  const dataSchemas = { ...knownDataSchemas };
  if (!Array.isArray(messages)) {
    return dataSchemas;
  }

  for (const message of messages) {
    if (!isRecord(message) || !Array.isArray(message.parts)) {
      continue;
    }

    for (const part of message.parts) {
      const name = dataPartName(part);
      if (name && dataSchemas[name] === undefined) {
        dataSchemas[name] = unknownDataSchema;
      }
    }
  }

  return dataSchemas;
}

export async function normalizeTranscriptMessages(messages: unknown): Promise<SessionMessage[]> {
  if (Array.isArray(messages) && messages.length === 0) {
    return [];
  }

  const capturedTurnMeta = capturePartTurnMeta(messages);
  const validationMessages = messagesForValidation(messages);
  const validated = await validateUIMessages<SessionMessage>({
    messages: validationMessages,
    dataSchemas: dataSchemasForMessages(validationMessages),
  });
  const withTurnMeta = reattachPartTurnMeta(validated, capturedTurnMeta);
  return restoreOriginalToolParts(withTurnMeta, messages);
}
