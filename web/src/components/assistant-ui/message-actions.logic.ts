interface MessageActionsState {
  /** The message's markdown text answer, joined and trimmed (empty for a pure tool turn). */
  source: string;
  /** Latest real event time across the message's parts, or null when none is recorded. */
  timestampMs: number | null;
  streaming: boolean;
  /** Copy is offered for non-streaming message text, including inspectable failure output. */
  visible: boolean;
  /** Goal prefill is offered only for terminal text without failure output. */
  goalEligible: boolean;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function isStreamingState(state: string | undefined): boolean {
  return state === "streaming" || state === "running";
}

function isAghEventPart(part: Record<string, unknown>): boolean {
  const type = stringField(part, "type");
  return (
    type === "data-agh-event" || (type === "data" && stringField(part, "name") === "agh-event")
  );
}

function isPersistedFailurePart(part: Record<string, unknown>): boolean {
  if (!isAghEventPart(part) || !isRecord(part.data)) {
    return false;
  }
  return (
    stringField(part.data, "type") === "error" ||
    stringField(part.data, "error") !== undefined ||
    isRecord(part.data.failure)
  );
}

function isRuntimeSystemBannerPart(part: Record<string, unknown>): boolean {
  if (!isAghEventPart(part) || !isRecord(part.data)) {
    return false;
  }
  return (
    stringField(part.data, "type") === "system" &&
    (stringField(part.data, "title") === "AGH Runtime Agent" ||
      stringField(part.data, "tool_name") === "AGH Runtime Agent")
  );
}

function isCanonicalRetriableFailureText(source: string): boolean {
  return /^Error:\s+RetriableError:\s+\S/.test(source);
}

function partTimestampMs(part: Record<string, unknown>): number | null {
  const own = stringField(part, "timestamp");
  const nested = isRecord(part.data) ? stringField(part.data, "timestamp") : undefined;
  const raw = own ?? nested;
  if (!raw) return null;
  const parsed = Date.parse(raw);
  return Number.isNaN(parsed) ? null : parsed;
}

/**
 * Derives copy and Goal action state from a thread message. Copy remains
 * available for non-streaming text, while Goal requires independently
 * classified terminal success.
 */
export function deriveMessageActions(message: {
  content?: unknown;
  status?: { type?: string };
}): MessageActionsState {
  const parts = Array.isArray(message.content) ? message.content : [];
  const textSegments: string[] = [];
  let streaming = message.status?.type === "running";
  let persistedFailure = false;
  let runtimeSystemBanner = false;
  let timestampMs: number | null = null;

  for (const part of parts) {
    if (!isRecord(part)) continue;
    const type = stringField(part, "type");
    if ((type === "text" || type === "reasoning") && isStreamingState(stringField(part, "state"))) {
      streaming = true;
    }
    if (type === "text") {
      const text = stringField(part, "text");
      if (text) textSegments.push(text);
    }
    persistedFailure = persistedFailure || isPersistedFailurePart(part);
    runtimeSystemBanner = runtimeSystemBanner || isRuntimeSystemBannerPart(part);
    const timestamp = partTimestampMs(part);
    if (timestamp !== null && (timestampMs === null || timestamp > timestampMs)) {
      timestampMs = timestamp;
    }
  }

  const source = textSegments.join("\n\n").trim();
  const visible = source.length > 0 && !streaming;
  const goalEligible =
    visible &&
    message.status?.type !== "incomplete" &&
    !persistedFailure &&
    !(runtimeSystemBanner && isCanonicalRetriableFailureText(source));
  return { source, timestampMs, streaming, visible, goalEligible };
}
