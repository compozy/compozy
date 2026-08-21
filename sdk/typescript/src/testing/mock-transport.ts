import { MethodNotFoundError, ensureRPCError } from "../errors.js";
import type { JSONRPCID, JSONRPCRequestEnvelope } from "../types.js";
import type { TransportHandler, TransportLike } from "../transport.js";

interface RecordedRequest {
  id: JSONRPCID;
  method: string;
  params: unknown;
}

function abortReason(signal?: AbortSignal): unknown {
  return signal?.reason ?? new DOMException("The operation was aborted", "AbortError");
}

export class MockTransport implements TransportLike {
  private readonly handlers = new Map<string, TransportHandler>();
  private readonly errors = new Set<(error: Error) => void>();
  private nextID = 0;
  private started = false;
  private closed = false;
  private peer: MockTransport | undefined;

  public readonly requests: RecordedRequest[] = [];

  public connect(peer: MockTransport): void {
    this.peer = peer;
  }

  public start(): void {
    this.started = true;
  }

  public handle(method: string, handler: TransportHandler): void {
    this.handlers.set(method.trim(), handler);
  }

  public unhandle(method: string): void {
    this.handlers.delete(method.trim());
  }

  public onTransportError(listener: (error: Error) => void): () => void {
    this.errors.add(listener);
    return () => {
      this.errors.delete(listener);
    };
  }

  public async close(): Promise<void> {
    this.closed = true;
  }

  public async call<TResult = unknown>(
    method: string,
    params?: unknown,
    signal?: AbortSignal
  ): Promise<TResult> {
    if (this.closed) {
      throw new Error("transport closed");
    }
    const peer = this.peer;
    if (!peer) {
      throw new Error("mock transport peer is not connected");
    }

    this.start();
    peer.start();

    const id = ++this.nextID;
    this.requests.push({ id, method, params });

    const handler = peer.handlers.get(method.trim());
    if (!handler) {
      throw new MethodNotFoundError(method);
    }

    const envelope: JSONRPCRequestEnvelope = {
      jsonrpc: "2.0",
      id,
      method,
      params,
    };
    const controller = new AbortController();
    const onAbort = (): void => {
      controller.abort(signal?.reason);
    };
    if (signal?.aborted) {
      onAbort();
    } else {
      signal?.addEventListener("abort", onAbort, { once: true });
    }

    let resultPromise: Promise<unknown> | undefined;
    try {
      if (controller.signal.aborted) {
        throw abortReason(signal);
      }
      resultPromise = Promise.resolve(handler(params, envelope, controller.signal));
      const abortPromise = new Promise<never>((_, reject) => {
        const rejectAborted = (): void => {
          reject(abortReason(signal));
        };
        if (controller.signal.aborted) {
          rejectAborted();
          return;
        }
        controller.signal.addEventListener("abort", rejectAborted, { once: true });
      });
      const result = await Promise.race([resultPromise, abortPromise]);
      if (controller.signal.aborted) {
        void resultPromise.catch(() => undefined);
        throw abortReason(signal);
      }
      return result as TResult;
    } catch (error) {
      void resultPromise?.catch(() => undefined);
      if (controller.signal.aborted) {
        throw abortReason(signal);
      }
      const rpcError = ensureRPCError(error);
      for (const listener of this.errors) {
        listener(rpcError);
      }
      throw rpcError;
    } finally {
      signal?.removeEventListener("abort", onAbort);
    }
  }
}

export function createMockTransportPair(): {
  host: MockTransport;
  extension: MockTransport;
} {
  const host = new MockTransport();
  const extension = new MockTransport();
  host.connect(extension);
  extension.connect(host);
  return { host, extension };
}
