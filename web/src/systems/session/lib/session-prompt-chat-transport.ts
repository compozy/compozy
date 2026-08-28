import { AssistantChatTransport } from "@assistant-ui/react-ai-sdk";
import type { UIMessage } from "ai";

import { createClientId } from "@/lib/client-id";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import { attachmentsFromPromptMessageParts } from "./attachment-kinds";
import type { SessionPromptRequest } from "../types";

interface SessionPromptChatTransportOptions {
  api: string;
  fetch: typeof globalThis.fetch;
  getRuntimeSnapshot?: () => SessionPromptRuntimeSnapshot | null;
  idempotencyKeys?: Map<string, string>;
  onPromptPrepared?: (request: { messages: readonly UIMessage[] }) => void;
  prepareUserMessage?: (message: UIMessage) => UIMessage;
  preparedUserMessages?: Map<string, UIMessage>;
}

type SessionPromptRequestBody = Omit<SessionPromptRequest, "messages"> & {
  messages: UIMessage[];
};

function latestUserMessage(messages: readonly UIMessage[]): UIMessage {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message?.role === "user" && message.id.trim().length > 0) {
      return message;
    }
  }
  throw new Error("A session prompt requires a user message with a durable id");
}

/**
 * Owns the browser-side prompt identity boundary. The assistant runtime creates
 * the optimistic user message; its ID is the durable message_id. This transport
 * mints one idempotency key and one prepared body per submission and reuses both
 * when the same message is retried.
 */
export function createSessionPromptChatTransport({
  api,
  fetch,
  getRuntimeSnapshot,
  idempotencyKeys = new Map<string, string>(),
  onPromptPrepared,
  prepareUserMessage,
  preparedUserMessages = new Map<string, UIMessage>(),
}: SessionPromptChatTransportOptions): AssistantChatTransport<UIMessage> {
  return new AssistantChatTransport<UIMessage>({
    api,
    fetch,
    prepareSendMessagesRequest: ({ messages }) => {
      const raw = latestUserMessage(messages);
      const message = preparedUserMessages.get(raw.id) ?? prepareUserMessage?.(raw) ?? raw;
      preparedUserMessages.set(message.id, message);
      const messageId = message.id;
      const idempotencyKey = idempotencyKeys.get(messageId) ?? createClientId();
      idempotencyKeys.set(messageId, idempotencyKey);
      const runtime = getRuntimeSnapshot?.() ?? null;
      const attachments = attachmentsFromPromptMessageParts(message.parts);
      const body: SessionPromptRequestBody = {
        idempotency_key: idempotencyKey,
        message_id: messageId,
        messages: [message],
        ...(attachments.length > 0 ? { attachments } : {}),
        ...(runtime ? { runtime } : {}),
      };
      onPromptPrepared?.({ messages: [message] });
      return { body };
    },
  });
}
