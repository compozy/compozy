import { describe, expect, it, vi } from "vitest";

import { createSessionPromptChatTransport } from "../session-prompt-chat-transport";

const userMessage = (id: string, text = "Continue the durable transcript") => ({
  id,
  parts: [{ text, type: "text" as const }],
  role: "user" as const,
});

function streamResponse() {
  return new Response(
    [
      'data: {"type":"start","messageId":"prompt-transport-turn"}\n\n',
      'data: {"type":"finish","finishReason":"stop"}\n\n',
      "data: [DONE]\n\n",
    ].join(""),
    {
      headers: { "content-type": "text/event-stream" },
    }
  );
}

describe("session prompt chat transport", () => {
  it("sends the optimistic user id as message_id and retains one idempotency key for retries", async () => {
    const fetch = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(
      async () => streamResponse()
    );
    const transport = createSessionPromptChatTransport({ api: "/prompt", fetch });
    const messages = [userMessage("message-001")];

    await transport.sendMessages({
      abortSignal: undefined,
      chatId: "session-001",
      messageId: undefined,
      messages,
      trigger: "submit-message",
    });
    await transport.sendMessages({
      abortSignal: undefined,
      chatId: "session-001",
      messageId: undefined,
      messages,
      trigger: "submit-message",
    });

    const first = JSON.parse(String(fetch.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
    const second = JSON.parse(String(fetch.mock.calls[1]?.[1]?.body)) as Record<string, unknown>;
    expect(first).toMatchObject({
      message_id: "message-001",
      messages,
    });
    expect(typeof first.idempotency_key).toBe("string");
    expect(first.idempotency_key).toBe(second.idempotency_key);
    expect(first).not.toHaveProperty("message");
    expect(first).not.toHaveProperty("messageId");
  });

  it("mints a separate key for the next submitted user message and retains the runtime snapshot", async () => {
    const fetch = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(
      async () => streamResponse()
    );
    const runtime = { provider: "codex", reasoning_effort: "high" as const };
    const transport = createSessionPromptChatTransport({
      api: "/prompt",
      fetch,
      getRuntimeSnapshot: () => runtime,
    });

    await transport.sendMessages({
      abortSignal: undefined,
      chatId: "session-001",
      messageId: undefined,
      messages: [userMessage("message-001")],
      trigger: "submit-message",
    });
    await transport.sendMessages({
      abortSignal: undefined,
      chatId: "session-001",
      messageId: undefined,
      messages: [userMessage("message-002")],
      trigger: "submit-message",
    });

    const first = JSON.parse(String(fetch.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
    const second = JSON.parse(String(fetch.mock.calls[1]?.[1]?.body)) as Record<string, unknown>;
    expect(first.idempotency_key).not.toBe(second.idempotency_key);
    expect(second).toMatchObject({ message_id: "message-002", runtime });
  });
});
