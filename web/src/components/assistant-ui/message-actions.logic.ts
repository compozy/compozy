interface MessageActionsState {
  /** The message's copyable text, joined and trimmed (empty for a pure tool turn). */
  source: string;
  /** Latest real event time across the message's parts, or null when none is recorded. */
  timestampMs: number | null;
  streaming: boolean;
  /** Copy stays in the row while streaming, including before the first text token arrives. */
  visible: boolean;
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

function copyableTextSegments(content: unknown): string[] {
  if (typeof content === "string") {
    return [content];
  }

  if (!Array.isArray(content)) {
    return [];
  }

  return content.flatMap(part => {
    if (typeof part === "string") {
      return [part];
    }
    if (!isRecord(part) || stringField(part, "type") !== "text") {
      return [];
    }

    const text = stringField(part, "text");
    return text ? [text] : [];
  });
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
 * Derives copy and timestamp state from a thread message.
 */
export function deriveMessageActions(message: {
  content?: unknown;
  status?: { type?: string };
}): MessageActionsState {
  const parts = Array.isArray(message.content) ? message.content : [];
  const textSegments = copyableTextSegments(message.content);
  let streaming = message.status?.type === "running";
  let timestampMs: number | null = null;

  for (const part of parts) {
    if (!isRecord(part)) continue;
    const type = stringField(part, "type");
    if ((type === "text" || type === "reasoning") && isStreamingState(stringField(part, "state"))) {
      streaming = true;
    }
    const timestamp = partTimestampMs(part);
    if (timestamp !== null && (timestampMs === null || timestamp > timestampMs)) {
      timestampMs = timestamp;
    }
  }

  const source = textSegments.join("\n\n").trim();
  // Streaming messages copy the text settled so far. Keeping the action in the
  // row prevents it from appearing only after generation finishes and shifting
  // the transcript layout.
  const visible = source.length > 0 || streaming;
  return { source, timestampMs, streaming, visible };
}
