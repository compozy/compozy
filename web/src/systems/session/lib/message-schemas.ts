import { validateUIMessages } from "ai";
import { z } from "zod";

import { capturePartMetadata, reattachPartMetadata } from "./message-part-metadata";
import { toolPartErrorText } from "./tool-part-error";

import type { SessionMessage } from "../types";

const compozyEventDataSchema = z.looseObject({
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
      meta: z.record(z.string(), z.unknown()).optional(),
      sequence: z.number().optional(),
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
  delivery: z
    .object({
      turn_id: z.string(),
      sent_at: z.string(),
      estimate: z.string(),
      // A delivery that sent no sections (a literal maintenance prompt such as `/compact`)
      // arrives with `spans: null`. It is quiet telemetry: it must never reject the frame
      // that carries the turn's reply.
      spans: z
        .array(
          z.object({
            key: z.string(),
            kind: z.string(),
            bytes: z.number(),
            tokens: z.number().nullable().optional(),
            delivery: z.string().optional(),
            name: z.string().optional(),
            unchanged: z.boolean().optional(),
            startup_dedup: z.boolean().optional(),
            hook_modified: z.boolean().optional(),
          })
        )
        .nullable(),
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

const compozyPermissionDataSchema = compozyEventDataSchema.extend({
  request_id: z.string(),
  raw: z.record(z.string(), z.unknown()).optional(),
});

// `status` is a free string: the four known values or a vendor value passed through
// verbatim, so an agent's own status never fails the whole transcript.
const compozyCompactionDataSchema = z.looseObject({
  kind: z.string(),
  compaction_id: z.string(),
  status: z.string(),
  summary: z.string().optional(),
  error: z.string().optional(),
  started_at: z.string(),
  ended_at: z.string().optional(),
});

// `kind` and `payload` stay open: a report kind this client doesn't know keeps its transcript.
const compozyReportDataSchema = z.looseObject({
  report_id: z.string(),
  instance_id: z.string(),
  kind: z.string(),
  payload: z.unknown(),
  created_at: z.string(),
});

const unknownDataSchema = z.unknown();

const knownDataSchemas: Record<string, z.ZodType<unknown>> = {
  "compozy-event": compozyEventDataSchema,
  "compozy-permission": compozyPermissionDataSchema,
  "compozy-compaction": compozyCompactionDataSchema,
  "compozy-report": compozyReportDataSchema,
};

type SessionMessagePart = NonNullable<SessionMessage["parts"]>[number];

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

function isDynamicToolPart(part: unknown): part is Record<string, unknown> {
  return isRecord(part) && part.type === "dynamic-tool";
}

function toolNameForValidation(part: Record<string, unknown>, type: string): string {
  const typedName = type.startsWith("tool-") ? type.slice("tool-".length).trim() : "";
  return stringField(part, "toolName") ?? (typedName || "tool");
}

function toolCallIdForValidation(part: Record<string, unknown>, toolName: string): string {
  return (
    stringField(part, "toolCallId") ??
    stringField(part, "tool_call_id") ??
    stringField(part, "id") ??
    `${toolName}-call`
  );
}

function normalizeToolPartForValidation(part: unknown): unknown {
  if (!isToolPart(part) && !isDynamicToolPart(part)) {
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
      validationPart.errorText = toolPartErrorText(part);
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

async function validationFails(message: unknown): Promise<boolean> {
  try {
    await validateMessages([message]);
    return false;
  } catch {
    return true;
  }
}

const ENVELOPE_PROBE_PART = { type: "text", text: "" };

interface SalvagedMessages {
  messages: unknown[];
  /** Original index of each salvaged message; a message emptied by dropping its parts is gone. */
  kept: number[];
}

/**
 * Drops the parts that still fail validation so one malformed part costs that part,
 * not the whole conversation, and drops a message every part of which was invalid.
 * Returns null when dropping parts salvages nothing (a malformed message envelope),
 * so the caller rethrows the original error.
 */
async function withoutInvalidParts(messages: readonly unknown[]): Promise<SalvagedMessages | null> {
  let dropped = false;
  const salvaged: SalvagedMessages = { messages: [], kept: [] };
  for (const [messageIndex, message] of messages.entries()) {
    if (!(await validationFails(message))) {
      salvaged.messages.push(message);
      salvaged.kept.push(messageIndex);
      continue;
    }
    // A malformed envelope (role, id) fails every per-part check too; reject it here
    // rather than mistaking it for a message whose parts were all invalid.
    if (
      !isRecord(message) ||
      !Array.isArray(message.parts) ||
      (await validationFails({ ...message, parts: [ENVELOPE_PROBE_PART] }))
    ) {
      return null;
    }
    const parts: unknown[] = [];
    for (const [index, part] of message.parts.entries()) {
      if (await validationFails({ ...message, parts: [part] })) {
        dropped = true;
        console.warn("Dropped an invalid transcript part", {
          messageId: message.id,
          partIndex: index,
          partType: isRecord(part) ? part.type : typeof part,
        });
        continue;
      }
      parts.push(part);
    }
    if (parts.length === 0 && message.parts.length > 0) {
      continue;
    }
    const next = { ...message, parts };
    if (await validationFails(next)) {
      return null;
    }
    salvaged.messages.push(next);
    salvaged.kept.push(messageIndex);
  }
  return dropped ? salvaged : null;
}

async function validateMessages(messages: unknown): Promise<SessionMessage[]> {
  const capturedMetadata = capturePartMetadata(messages);
  const validationMessages = messagesForValidation(messages);
  const validated = await validateUIMessages<SessionMessage>({
    messages: validationMessages,
    dataSchemas: dataSchemasForMessages(validationMessages),
  });
  const withMetadata = reattachPartMetadata(validated, capturedMetadata);
  return restoreOriginalToolParts(withMetadata, messages);
}

/**
 * Validates the messages of transcript entries. An entry whose every part was invalid
 * is left out, so the result can be shorter than the input but keeps its order.
 */
export async function normalizeTranscriptEntries<T extends { message: unknown }>(
  entries: readonly T[]
): Promise<Array<Omit<T, "message"> & { message: SessionMessage }>> {
  if (entries.length === 0) {
    return [];
  }

  const messages = entries.map(entry => entry.message);
  try {
    const validated = await validateMessages(messages);
    return entries.map((entry, index) => ({ ...entry, message: validated[index]! }));
  } catch (error) {
    const salvaged = await withoutInvalidParts(messages);
    if (!salvaged) {
      throw error;
    }
    if (salvaged.messages.length === 0) {
      return [];
    }
    const validated = await validateMessages(salvaged.messages);
    return salvaged.kept.map((entryIndex, index) => ({
      ...entries[entryIndex]!,
      message: validated[index]!,
    }));
  }
}

export async function normalizeTranscriptMessages(messages: unknown): Promise<SessionMessage[]> {
  if (!Array.isArray(messages)) {
    return validateMessages(messages);
  }
  const entries = await normalizeTranscriptEntries(messages.map(message => ({ message })));
  return entries.map(entry => entry.message);
}
