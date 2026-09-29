import type { ThreadMessage } from "@assistant-ui/react";

import { isAgentEventPayload } from "./message-parts";
import { isProviderFailureRecord } from "./provider-error";
import type { AgentEventPayload } from "../types";

function transcriptMessageIDs(transcriptMessages: readonly ThreadMessage[]): ReadonlySet<string> {
  return new Set(transcriptMessages.map(message => message.id));
}

export function hasUnreconciledRuntimeMessages({
  transcriptMessages,
  runtimeMessages,
}: {
  transcriptMessages: readonly ThreadMessage[];
  runtimeMessages: readonly ThreadMessage[];
}): boolean {
  const transcriptIDs = transcriptMessageIDs(transcriptMessages);
  return runtimeMessages.some(message => !transcriptIDs.has(message.id));
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

  const transcriptIDs = transcriptMessageIDs(transcriptMessages);
  const recordedFailureTurns = providerFailureTurnIDs(transcriptMessages);
  const optimisticTail: ThreadMessage[] = [];

  for (const message of runtimeMessages) {
    if (transcriptIDs.has(message.id)) continue;
    if (isRecordedProviderFailure(message, recordedFailureTurns)) continue;
    optimisticTail.push(message);
  }

  return optimisticTail.length === 0
    ? transcriptMessages
    : [...transcriptMessages, ...optimisticTail];
}

function compozyEvent(part: unknown): AgentEventPayload | null {
  if (typeof part !== "object" || part === null) return null;
  const record = part as { type?: unknown; name?: unknown; data?: unknown };
  const isEvent =
    record.type === "data-compozy-event" ||
    (record.type === "data" &&
      (record.name === "compozy-event" || record.name === "data-compozy-event"));
  return isEvent && isAgentEventPayload(record.data) ? record.data : null;
}

function providerFailureTurnID(part: unknown): string | null {
  const event = compozyEvent(part);
  if (!event || !isProviderFailureRecord(event)) return null;
  return event.turn_id?.trim() || null;
}

function providerFailureTurnIDs(messages: readonly ThreadMessage[]): ReadonlySet<string> {
  const turnIDs = new Set<string>();
  for (const message of messages) {
    for (const part of message.content) {
      const turnID = providerFailureTurnID(part);
      if (turnID) turnIDs.add(turnID);
    }
  }
  return turnIDs;
}

/**
 * A failed turn's live stream message carries only the provider failure and has
 * its own id, so id reconciliation never matches it to the durable record. When
 * the transcript already records a failure for that turn, the live copy is the
 * same failure: appending it would render the notice twice.
 */
function isRecordedProviderFailure(
  message: ThreadMessage,
  recordedFailureTurns: ReadonlySet<string>
): boolean {
  if (recordedFailureTurns.size === 0 || message.content.length === 0) return false;
  return message.content.every(part => {
    const turnID = providerFailureTurnID(part);
    return turnID !== null && recordedFailureTurns.has(turnID);
  });
}
