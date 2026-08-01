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

  it("preserves a shared idempotency key when the transport is recreated", async () => {
    const fetch = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(
      async () => streamResponse()
    );
    const idempotencyKeys = new Map<string, string>();
    const messages = [userMessage("message-001")];
    const firstTransport = createSessionPromptChatTransport({
      api: "/prompt",
      fetch,
      idempotencyKeys,
    });

    await firstTransport.sendMessages({
      abortSignal: undefined,
      chatId: "session-001",
      messageId: undefined,
      messages,
      trigger: "submit-message",
    });

    const recreatedTransport = createSessionPromptChatTransport({
      api: "/prompt",
      fetch,
      idempotencyKeys,
    });
    await recreatedTransport.sendMessages({
      abortSignal: undefined,
      chatId: "session-001",
      messageId: undefined,
      messages,
      trigger: "submit-message",
    });

    const first = JSON.parse(String(fetch.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
    const retry = JSON.parse(String(fetch.mock.calls[1]?.[1]?.body)) as Record<string, unknown>;
    expect(first.idempotency_key).toEqual(expect.stringMatching(/\S/));
    expect(retry.idempotency_key).toBe(first.idempotency_key);
  });

  it("mints an idempotency key from cryptographic random bytes without randomUUID", async () => {
    const getRandomValues = vi.fn((bytes: Uint8Array) => {
      bytes.set(
        Uint8Array.from([
          0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee,
          0xff,
        ])
      );
      return bytes;
    });
    vi.stubGlobal("crypto", { getRandomValues });
    const fetch = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(
      async () => streamResponse()
    );
    const transport = createSessionPromptChatTransport({ api: "/prompt", fetch });

    try {
      await transport.sendMessages({
        abortSignal: undefined,
        chatId: "session-001",
        messageId: undefined,
        messages: [userMessage("message-001")],
        trigger: "submit-message",
      });

      const body = JSON.parse(String(fetch.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
      expect(getRandomValues).toHaveBeenCalledWith(expect.any(Uint8Array));
      expect(body.idempotency_key).toBe("00112233-4455-4677-8899-aabbccddeeff");
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
