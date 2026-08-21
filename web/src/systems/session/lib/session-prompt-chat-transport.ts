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
 * the optimistic user message; its ID is the durable message_id, while this
 * transport mints exactly one idempotency key for that submission and reuses it
 * whenever the transport retries the same message.
 */
export function createSessionPromptChatTransport({
  api,
  fetch,
  getRuntimeSnapshot,
  idempotencyKeys = new Map<string, string>(),
  onPromptPrepared,
}: SessionPromptChatTransportOptions): AssistantChatTransport<UIMessage> {
  return new AssistantChatTransport<UIMessage>({
    api,
    fetch,
    prepareSendMessagesRequest: ({ messages }) => {
      const message = latestUserMessage(messages);
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
