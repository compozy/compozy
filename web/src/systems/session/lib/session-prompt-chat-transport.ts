import { AssistantChatTransport } from "@assistant-ui/react-ai-sdk";
import type { UIMessage } from "ai";

import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";

interface SessionPromptChatTransportOptions {
  api: string;
  fetch: typeof globalThis.fetch;
  getRuntimeSnapshot?: () => SessionPromptRuntimeSnapshot | null;
  idempotencyKeys?: Map<string, string>;
}

interface SessionPromptRequestBody {
  idempotency_key: string;
  message_id: string;
  messages: UIMessage[];
  runtime?: SessionPromptRuntimeSnapshot;
}

function latestUserMessage(messages: readonly UIMessage[]): UIMessage {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message?.role === "user" && message.id.trim().length > 0) {
      return message;
    }
  }
  throw new Error("A session prompt requires a user message with a durable id");
}

function createIdempotencyKey(): string {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.randomUUID === "function") {
    return cryptoApi.randomUUID();
  }
  if (typeof cryptoApi?.getRandomValues !== "function") {
    throw new Error("Browser cryptography is required to create a session prompt idempotency key");
  }
  const bytes = cryptoApi.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/**
 * Owns the browser-side prompt identity boundary. The assistant runtime creates
 * the optimistic user message; its ID is the durable message_id, while this
 * transport mints exactly one idempotency key for that submission and reuses it
 * whenever the transport retries the same message.
 */
export function createSessionPromptChatTransport({
  api,
  fetch,
  getRuntimeSnapshot,
  idempotencyKeys = new Map<string, string>(),
}: SessionPromptChatTransportOptions): AssistantChatTransport<UIMessage> {
  return new AssistantChatTransport<UIMessage>({
    api,
    fetch,
    prepareSendMessagesRequest: ({ messages }) => {
      const message = latestUserMessage(messages);
      const messageId = message.id;
      const idempotencyKey = idempotencyKeys.get(messageId) ?? createIdempotencyKey();
      idempotencyKeys.set(messageId, idempotencyKey);
      const runtime = getRuntimeSnapshot?.() ?? null;
      const body: SessionPromptRequestBody = {
        idempotency_key: idempotencyKey,
        message_id: messageId,
        messages,
        ...(runtime ? { runtime } : {}),
      };
      return { body };
    },
  });
}
