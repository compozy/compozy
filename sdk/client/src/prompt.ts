import type { ConnectedState } from "./connect.js";
import { CompozyStreamError, responseError } from "./errors.js";
import { createSseClient } from "./generated/sdk/core/serverSentEvents.gen.js";
import type { OperationRequestBody, OperationResponse } from "./operation-types.js";

export interface PromptInput {
  workspaceId: string;
  sessionId: string;
  message: string;
  mode?: "queue" | "interrupt" | "steer";
  idempotencyKey?: string;
  expectedTurnId?: string;
  signal?: AbortSignal;
}

type JSONValue =
  | null
  | string
  | number
  | boolean
  | JSONValue[]
  | { [key: string]: JSONValue | undefined };
type ProviderMetadata = Record<string, Record<string, JSONValue>>;
type ToolMetadata = {
  providerExecuted?: boolean;
  providerMetadata?: ProviderMetadata;
  dynamic?: boolean;
  title?: string;
};

/** Wire-compatible UI message chunks, without a runtime or type dependency on `ai`. */
export type UIMessageChunk =
  | {
      type: "text-start" | "text-end" | "reasoning-start" | "reasoning-end";
      id: string;
      providerMetadata?: ProviderMetadata;
    }
  | {
      type: "text-delta" | "reasoning-delta";
      id: string;
      delta: string;
      providerMetadata?: ProviderMetadata;
    }
  | ({ type: "tool-input-start"; toolCallId: string; toolName: string } & ToolMetadata)
  | { type: "tool-input-delta"; toolCallId: string; inputTextDelta: string }
  | ({
      type: "tool-input-available";
      toolCallId: string;
      toolName: string;
      input: unknown;
    } & ToolMetadata)
  | ({
      type: "tool-input-error";
      toolCallId: string;
      toolName: string;
      input: unknown;
      errorText: string;
    } & ToolMetadata)
  | ({
      type: "tool-output-available";
      toolCallId: string;
      output: unknown;
      preliminary?: boolean;
    } & ToolMetadata)
  | ({ type: "tool-output-error"; toolCallId: string; errorText: string } & ToolMetadata)
  | { type: "tool-output-denied"; toolCallId: string }
  | { type: "tool-approval-request"; approvalId: string; toolCallId: string }
  | { type: "tool-approval-response"; approvalId: string; approved: boolean; reason?: string }
  | {
      type: "source-url";
      sourceId: string;
      url: string;
      title?: string;
      providerMetadata?: ProviderMetadata;
    }
  | {
      type: "source-document";
      sourceId: string;
      mediaType: string;
      title: string;
      filename?: string;
      providerMetadata?: ProviderMetadata;
    }
  | {
      type: "file" | "reasoning-file";
      url: string;
      mediaType: string;
      providerMetadata?: ProviderMetadata;
    }
  | { type: `data-${string}`; id?: string; data: unknown; transient?: boolean }
  | { type: "start-step" | "finish-step" | "reset-step" }
  | { type: "start"; messageId?: string; messageMetadata?: unknown }
  | {
      type: "finish";
      finishReason?: "stop" | "length" | "content-filter" | "tool-calls" | "error" | "other";
      messageMetadata?: unknown;
    }
  | { type: "abort"; reason?: string }
  | { type: "error"; errorText: string }
  | { type: "message-metadata"; messageMetadata: unknown };

export type PromptResult = OperationResponse<"sendSessionPrompt", 202>["prompt"];
export type PromptReply =
  | { kind: "accepted"; result: PromptResult }
  | {
      kind: "stream";
      chunks: AsyncIterable<UIMessageChunk>;
      toReadableStream(): ReadableStream<UIMessageChunk>;
    };

export async function promptResponse(
  state: ConnectedState,
  input: PromptInput,
  createId = createPromptId
): Promise<Response> {
  const response = await sendPrompt(state, input, createId);
  if (!response.ok || isStream(response) || response.status === 202) return response;
  // Queued, steered, and goal-command results all use the accepted proxy shape.
  return new Response(response.body, { status: 202, headers: response.headers });
}

export async function prompt(
  state: ConnectedState,
  input: PromptInput,
  createId = createPromptId
): Promise<PromptReply> {
  const response = await sendPrompt(state, input, createId);
  if (!response.ok) throw await responseError(response);
  if (!isStream(response)) {
    const body: unknown = await response.json();
    if (
      !body ||
      typeof body !== "object" ||
      !("prompt" in body) ||
      !body.prompt ||
      typeof body.prompt !== "object"
    ) {
      throw new CompozyStreamError("Prompt response is missing its accepted result.");
    }
    return { kind: "accepted", result: body.prompt as PromptResult };
  }
  const chunks = promptChunks(response, input.signal);
  return { kind: "stream", chunks, toReadableStream: () => readable(chunks) };
}

function sendPrompt(
  state: ConnectedState,
  input: PromptInput,
  createId: () => string
): Promise<Response> {
  const key = input.idempotencyKey ?? createId();
  // Reusing a caller's key must preserve the message identity as well as the receipt key.
  const body: OperationRequestBody<"sendSessionPrompt"> = {
    message: input.message,
    message_id: key,
    mode: input.mode,
    idempotency_key: key,
    expected_turn_id: input.expectedTurnId,
  };
  const headers = new Headers(state.headers);
  headers.set("Content-Type", "application/json");
  headers.set("Accept", "text/event-stream, application/json");
  return state.fetch(
    `${state.baseUrl}/api/workspaces/${encodeURIComponent(input.workspaceId)}/sessions/${encodeURIComponent(input.sessionId)}/prompt`,
    {
      method: "POST",
      headers,
      signal: input.signal,
      body: JSON.stringify(body),
    }
  );
}

// Use the same UUID fallback as the Web client: getRandomValues also works on HTTP origins.
export function createPromptId(): string {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.randomUUID === "function") return cryptoApi.randomUUID();
  if (typeof cryptoApi?.getRandomValues !== "function") {
    throw new Error("Cryptography is required to create a prompt identifier.");
  }
  const bytes = cryptoApi.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function isStream(response: Response): boolean {
  return response.headers.get("content-type")?.split(";", 1)[0]?.trim() === "text/event-stream";
}

async function* promptChunks(
  response: Response,
  signal?: AbortSignal
): AsyncGenerator<UIMessageChunk> {
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  let failure: unknown;
  let parsedJSON = false;
  const source = response.body?.getReader();
  if (!source) throw new CompozyStreamError("Prompt stream has no body.");
  let tail = "";
  const body = new ReadableStream<Uint8Array>({
    async pull(destination) {
      const next = await source.read();
      if (next.done) {
        // The generated parser owns framing. Its current runtime silently discards
        // an unterminated tail, so guard only the final wire delimiter here.
        if (tail && !tail.replace(/\r\n/g, "\n").replace(/\r/g, "\n").endsWith("\n\n")) {
          destination.error(new CompozyStreamError("Prompt stream ended mid-frame."));
        } else destination.close();
      } else {
        tail = (tail + String.fromCharCode(...next.value.subarray(-4))).slice(-4);
        destination.enqueue(next.value);
      }
    },
    async cancel() {
      await source.cancel();
    },
  });
  const { stream } = createSseClient({
    url: response.url || "http://localhost/prompt",
    fetch: async () => new Response(body, { headers: response.headers }),
    signal: controller.signal,
    sseMaxRetryAttempts: 1,
    responseValidator: () => {
      parsedJSON = true;
      return Promise.resolve();
    },
    onSseEvent: ({ data }) => {
      if (data === undefined) return;
      if (data === "[DONE]") return;
      if (
        !parsedJSON ||
        !data ||
        typeof data !== "object" ||
        !("type" in data) ||
        typeof data.type !== "string"
      ) {
        throw new CompozyStreamError("Prompt stream contains a malformed UI message chunk.");
      }
      parsedJSON = false;
    },
    onSseError: error => {
      failure = error;
      controller.abort();
    },
  });
  try {
    for await (const data of stream) {
      if (signal?.aborted)
        throw new CompozyStreamError("Prompt stream was interrupted; the daemon turn continues.");
      if (data === "[DONE]") break;
      yield data as UIMessageChunk;
    }
    if (failure || signal?.aborted) {
      throw failure instanceof CompozyStreamError
        ? failure
        : new CompozyStreamError("Prompt stream was interrupted; the daemon turn continues.", {
            cause: failure,
          });
    }
  } finally {
    controller.abort();
    signal?.removeEventListener("abort", abort);
    await stream.return();
    await source.cancel().catch(() => {});
    source.releaseLock();
  }
}

function readable<T>(iterable: AsyncIterable<T>): ReadableStream<T> {
  const iterator = iterable[Symbol.asyncIterator]();
  return new ReadableStream({
    async pull(controller) {
      try {
        const next = await iterator.next();
        if (next.done) controller.close();
        else controller.enqueue(next.value);
      } catch (error) {
        controller.error(error);
      }
    },
    async cancel() {
      await iterator.return?.();
    },
  });
}
