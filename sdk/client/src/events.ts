import type { ConnectedState } from "./connect.js";
import { CompozyApiError, CompozyStreamError, responseError } from "./errors.js";
import { createClient } from "./generated/sse/client/index.js";
import type {
  ServerSentEventsOptions,
  ServerSentEventsResult,
  StreamEvent,
} from "./generated/sse/core/serverSentEvents.gen.js";
import * as operations from "./generated/sse/sdk.gen.js";

export type { StreamEvent } from "./generated/sse/core/serverSentEvents.gen.js";
export type SseOperationId = keyof typeof operations;
type GeneratedOptions<Id extends SseOperationId> = NonNullable<
  Parameters<(typeof operations)[Id]>[0]
>;
export type SseOptions<Id extends SseOperationId> = Pick<
  GeneratedOptions<Id>,
  Extract<keyof GeneratedOptions<Id>, "path" | "query" | "body" | "signal">
>;
export type SseFrame<Id extends SseOperationId> =
  Awaited<ReturnType<(typeof operations)[Id]>>["stream"] extends AsyncGenerator<infer Frame>
    ? Frame
    : never;

type InvokeOptions = Omit<ServerSentEventsOptions, "url"> & {
  client: ReturnType<typeof createClient>;
};
type InvokeStream = (options: InvokeOptions) => Promise<ServerSentEventsResult>;

/** All event feeds use the same bounded reconnect and typed-error policy. */
export async function* events<Id extends SseOperationId>(
  state: ConnectedState,
  operationId: Id,
  options: SseOptions<Id>
): AsyncGenerator<StreamEvent<SseFrame<Id>>> {
  const signal = options.signal ?? undefined;
  let failures = 0;
  let delay = 500;
  let lastEventId: string | undefined;
  while (!signal?.aborted) {
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort, { once: true });
    if (signal?.aborted) abort();
    let failure: unknown;
    let parsedJSON = false;
    let receivedFrame = false;
    let frame: StreamEvent<unknown> | undefined;
    let activeStream: AsyncGenerator<unknown> | undefined;
    const headers = new Headers(state.headers);
    headers.set("Accept", "text/event-stream");
    if (lastEventId !== undefined) headers.set("Last-Event-ID", lastEventId);
    const client = createClient({
      baseUrl: state.baseUrl,
      headers,
      fetch: async (input, init) => {
        const response = await state.fetch(input, init);
        if (!response.ok) throw await responseError(response);
        return response;
      },
    });
    try {
      // The operation id and its path/query remain correlated at the public boundary.
      const invoke = operations[operationId] as unknown as InvokeStream;
      const result = await invoke({
        ...options,
        client,
        signal: controller.signal,
        sseMaxRetryAttempts: 1,
        responseValidator: () => {
          parsedJSON = true;
          return Promise.resolve();
        },
        onSseEvent: received => {
          if (received.id !== undefined) lastEventId = received.id;
          if (received.data === undefined) return;
          if (!parsedJSON)
            throw new CompozyStreamError(`Malformed JSON frame from ${operationId}.`);
          parsedJSON = false;
          receivedFrame = true;
          failures = 0;
          delay = 500;
          frame = {
            data: received.data,
            ...(received.event !== undefined && { event: received.event }),
            ...(received.id !== undefined && { id: received.id }),
          };
        },
        onSseError: error => {
          failure = error;
          controller.abort();
        },
      });
      activeStream = result.stream;
      for await (const _data of activeStream) {
        if (signal?.aborted) return;
        if (frame) {
          yield frame as StreamEvent<SseFrame<Id>>;
          frame = undefined;
        }
      }
    } catch (error) {
      failure = error;
    } finally {
      controller.abort();
      signal?.removeEventListener("abort", abort);
      await activeStream?.return(undefined);
    }
    if (signal?.aborted) return;
    if (
      failure instanceof CompozyStreamError ||
      (failure instanceof CompozyApiError && failure.status >= 400 && failure.status < 500)
    )
      throw failure;
    if (!receivedFrame && ++failures >= 20) {
      throw new CompozyStreamError(`${operationId} failed after 20 consecutive stream attempts.`, {
        cause: failure,
      });
    }
    await backoff(delay, signal);
    delay = Math.min(delay * 2, 8000);
  }
}

function backoff(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise(resolve => {
    const finish = () => {
      clearTimeout(timer);
      signal?.removeEventListener("abort", finish);
      resolve();
    };
    const timer = setTimeout(finish, ms);
    signal?.addEventListener("abort", finish, { once: true });
    if (signal?.aborted) finish();
  });
}
