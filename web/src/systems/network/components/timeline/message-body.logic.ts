import type { NetworkConversationMessage } from "../../types";

interface MessageBody {
  text?: string;
  summary?: string;
  message?: string;
  detail?: string;
  query?: string;
}

export function readMessageBody(message: NetworkConversationMessage): string {
  const text =
    typeof message.text === "string" && message.text.trim().length > 0 ? message.text : null;
  if (text) {
    return text;
  }

  const body = (message.body ?? null) as MessageBody | null;
  const candidates: Array<string | undefined> = [
    body?.text,
    body?.summary,
    body?.message,
    body?.detail,
    body?.query,
    message.preview_text ?? undefined,
  ];
  for (const candidate of candidates) {
    if (typeof candidate === "string" && candidate.trim().length > 0) {
      return candidate;
    }
  }
  return "";
}
