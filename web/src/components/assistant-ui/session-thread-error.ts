import { isAgentEventPayload, isProviderErrorEvent } from "@/systems/session";

function textField(record: Record<string, unknown>, key: string): string | null {
  const value = record[key];
  if (typeof value !== "string") {
    return null;
  }
  const message = value.trim();
  return message.length > 0 ? message : null;
}

function recordField(record: Record<string, unknown>, key: string): Record<string, unknown> | null {
  const value = record[key];
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function messageFromErrorRecord(record: Record<string, unknown>): string | null {
  const data = recordField(record, "data");
  if (data) {
    const dataMessage = textField(data, "error") ?? textField(data, "message");
    if (dataMessage) {
      return dataMessage;
    }
  }

  const failure = recordField(record, "failure");
  if (failure) {
    const failureMessage = textField(failure, "summary") ?? textField(failure, "message");
    if (failureMessage) {
      return failureMessage;
    }
  }

  return (
    textField(record, "error") ??
    textField(record, "summary") ??
    textField(record, "detail") ??
    textField(record, "message")
  );
}

/**
 * Normalizes an assistant/transcript error into a human-readable line, or `null`
 * when there is no meaningful detail to show. Shared by the inline message error
 * notice and the transcript-fetch error pane so both surface the same provider text.
 */
export function formatMessageError(error: unknown): string | null {
  if (error instanceof Error) {
    return formatMessageError(error.message);
  }

  if (typeof error === "object" && error !== null && !Array.isArray(error)) {
    return messageFromErrorRecord(error as Record<string, unknown>);
  }

  if (typeof error === "string") {
    const message = error.trim();
    if (message.length === 0) {
      return null;
    }

    try {
      const parsed = JSON.parse(message) as unknown;
      return formatMessageError(parsed);
    } catch {
      // Non-JSON provider errors are already human-readable enough to display.
    }

    return message;
  }

  return null;
}

function compozyEventData(part: unknown): unknown {
  if (typeof part !== "object" || part === null) return null;
  const record = part as Record<string, unknown>;
  const isEvent =
    record.type === "data-compozy-event" ||
    (record.type === "data" && record.name === "compozy-event");
  return isEvent ? record.data : null;
}

/**
 * True when a provider-diagnostic error part in the same message already tells this
 * story: the live stream's `errorText` and the persisted event's `error` carry the
 * same daemon summary, so equal normalized text means one failure, not two.
 */
export function providerDiagnosticOwnsError(content: unknown, error: string): boolean {
  if (!Array.isArray(content)) return false;
  return content.some(part => {
    const data = compozyEventData(part);
    if (!isAgentEventPayload(data) || !isProviderErrorEvent(data)) return false;
    return formatMessageError(data.error ?? data.failure?.summary) === error;
  });
}
