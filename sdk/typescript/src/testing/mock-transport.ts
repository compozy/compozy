import { MethodNotFoundError, ensureRPCError } from "../errors.js";
import type { JSONRPCID, JSONRPCRequestEnvelope } from "../types.js";
import type { TransportHandler, TransportLike } from "../transport.js";

interface RecordedRequest {
  id: JSONRPCID;
  method: string;
  params: unknown;
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

  public onTransportError(listener: (error: Error) => void): () => void {
    this.errors.add(listener);
    return () => {
      this.errors.delete(listener);
    };
  }

  public async close(): Promise<void> {
    this.closed = true;
  }

  public async call<TResult = unknown>(method: string, params?: unknown): Promise<TResult> {
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

    try {
      return (await handler(params, envelope)) as TResult;
    } catch (error) {
      const rpcError = ensureRPCError(error);
      for (const listener of this.errors) {
        listener(rpcError);
      }
      throw rpcError;
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
