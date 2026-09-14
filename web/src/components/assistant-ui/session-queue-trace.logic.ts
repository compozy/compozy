import { isAgentEventPayload } from "@/systems/session/lib/message-parts";
import { toTimelineParts } from "@/systems/session/lib/timeline-message-parts";

interface QueueTraceMessage {
  id: string;
  role: string;
  content?: unknown;
}

function traceKind(message: QueueTraceMessage): string | null {
  if (message.role !== "assistant") return null;
  const parts = toTimelineParts(message);
  if (parts.length !== 1) return null;
  const part = parts[0]!;
  if (
    part.kind !== "data" ||
    part.name !== "data-compozy-event" ||
    !isAgentEventPayload(part.data)
  ) {
    return null;
  }
  if (part.data.type === "session.queue_cleared") return "sibling";
  const marker = part.data.marker;
  if (marker?.kind !== "transcript_marker.queue_cleared") return null;
  const evidence = marker.evidence;
  return JSON.stringify([evidence?.actor_kind, evidence?.actor_id, evidence?.actor_name]);
}

/** Only presentation is clustered; each durable message keeps its identity and position. */
export function queueClearTraceCount(
  messages: readonly QueueTraceMessage[],
  id: string
): number | null {
  const index = messages.findIndex(message => message.id === id);
  const message = messages[index];
  if (!message) return null;
  const kind = traceKind(message);
  if (kind === null || kind === "sibling") return null;
  for (let previous = index - 1; previous >= 0; previous -= 1) {
    const previousKind = traceKind(messages[previous]!);
    if (previousKind === kind) return 0;
    if (previousKind !== "sibling") break;
  }
  let count = 1;
  for (let next = index + 1; next < messages.length; next += 1) {
    const nextKind = traceKind(messages[next]!);
    if (nextKind === kind) count += 1;
    else if (nextKind !== "sibling") break;
  }
  return count;
}
