import { Buffer } from "node:buffer";
import type { Readable, Writable } from "node:stream";

import {
  InvalidRequestError,
  MethodNotFoundError,
  NotInitializedError,
  ParseError,
  ensureRPCError,
  errorFromObject,
} from "./errors.js";
import type { JSONRPCID, JSONRPCRequestEnvelope, JSONRPCResponseEnvelope } from "./types.js";

export const DEFAULT_MAX_MESSAGE_BYTES = 10 * 1024 * 1024;
export const JSON_RPC_VERSION = "2.0";

export type TransportHandler = (
  params: unknown,
  request: JSONRPCRequestEnvelope
) => Promise<unknown> | unknown;

export interface TransportLike {
  call<TResult = unknown>(method: string, params?: unknown): Promise<TResult>;
  handle(method: string, handler: TransportHandler): void;
  onTransportError(listener: (error: Error) => void): () => void;
  start(): void;
  close(): Promise<void>;
}

interface PendingCall {
  resolve: (value: any) => void;
  reject: (reason: unknown) => void;
}

interface StdioTransportOptions {
  input?: Readable;
  output?: Writable;
  maxMessageBytes?: number;
}

interface RequestFrame extends JSONRPCRequestEnvelope {}

interface ResponseFrame extends JSONRPCResponseEnvelope {}

function keyOfID(id: JSONRPCID): string {
  return `${typeof id}:${String(id)}`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isRequestFrame(value: unknown): value is RequestFrame {
  if (!isRecord(value)) {
    return false;
  }
  return value.jsonrpc === JSON_RPC_VERSION && typeof value.method === "string";
}

function isResponseFrame(value: unknown): value is ResponseFrame {
  if (!isRecord(value)) {
    return false;
  }
  return value.jsonrpc === JSON_RPC_VERSION && "id" in value && !("method" in value);
}

export class StdioTransport implements TransportLike {
  private readonly input: Readable;
  private readonly output: Writable;
  private readonly maxMessageBytes: number;
  private readonly handlers = new Map<string, TransportHandler>();
  private readonly pending = new Map<string, PendingCall>();
  private readonly errorListeners = new Set<(error: Error) => void>();
  private readBuffer = Buffer.alloc(0);
  private nextID = 0;
  private started = false;
  private closed = false;
  private readonly onDataBound: (chunk: Buffer | string) => void;
  private readonly onErrorBound: (error: Error) => void;
  private readonly onEndBound: () => void;

  public constructor(options: StdioTransportOptions = {}) {
    this.input = options.input ?? process.stdin;
    this.output = options.output ?? process.stdout;
    this.maxMessageBytes = options.maxMessageBytes ?? DEFAULT_MAX_MESSAGE_BYTES;
    this.onDataBound = chunk => {
      this.handleChunk(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
    };
    this.onErrorBound = error => {
      this.fail(error);
    };
    this.onEndBound = () => {
      this.fail(new Error("transport closed"));
    };
  }

  public start(): void {
    if (this.started) {
      return;
    }
    this.started = true;
    this.input.on("data", this.onDataBound);
    this.input.on("error", this.onErrorBound);
    this.input.on("end", this.onEndBound);
    this.input.on("close", this.onEndBound);
  }

  public handle(method: string, handler: TransportHandler): void {
    this.handlers.set(method.trim(), handler);
  }

  public onTransportError(listener: (error: Error) => void): () => void {
    this.errorListeners.add(listener);
    return () => {
      this.errorListeners.delete(listener);
    };
  }

  public async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.fail(new Error("transport closed"));
  }

  public async call<TResult = unknown>(method: string, params?: unknown): Promise<TResult> {
    if (this.closed) {
      throw new Error("transport closed");
    }
    this.start();

    const id = ++this.nextID;
    const frame: RequestFrame = {
      jsonrpc: JSON_RPC_VERSION,
      id,
      method,
      params,
    };

    return await new Promise<TResult>((resolve, reject) => {
      this.pending.set(keyOfID(id), { resolve, reject });
      try {
        this.writeFrame(frame);
      } catch (error) {
        this.pending.delete(keyOfID(id));
        reject(error);
      }
    });
  }

  private handleChunk(chunk: Buffer): void {
    if (this.closed) {
      return;
    }

    this.readBuffer = Buffer.concat([this.readBuffer, chunk]);
    if (this.readBuffer.length > this.maxMessageBytes + 1) {
      this.fail(new Error(`message exceeds ${this.maxMessageBytes} bytes`));
      return;
    }

    while (true) {
      const newlineIndex = this.readBuffer.indexOf(0x0a);
      if (newlineIndex === -1) {
        if (this.readBuffer.length > this.maxMessageBytes) {
          this.fail(new Error(`message exceeds ${this.maxMessageBytes} bytes`));
        }
        return;
      }

      const line = this.readBuffer.subarray(0, newlineIndex);
      this.readBuffer = this.readBuffer.subarray(newlineIndex + 1);

      const trimmed = line.toString("utf8").trim();
      if (trimmed.length === 0) {
        continue;
      }
      if (Buffer.byteLength(trimmed) > this.maxMessageBytes) {
        this.fail(new Error(`message exceeds ${this.maxMessageBytes} bytes`));
        return;
      }
      this.processLine(trimmed);
      if (this.closed) {
        return;
      }
    }
  }

  private processLine(line: string): void {
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch (error) {
      this.fail(
        error instanceof Error ? new ParseError(error.message) : new ParseError(String(error))
      );
      return;
    }

    if (Array.isArray(parsed)) {
      this.fail(new InvalidRequestError("batch requests are not supported"));
      return;
    }
    if (isRequestFrame(parsed)) {
      if (!("id" in parsed) || parsed.id === undefined || parsed.id === null) {
        return;
      }
      void this.dispatchRequest(parsed);
      return;
    }
    if (isResponseFrame(parsed)) {
      this.dispatchResponse(parsed);
      return;
    }

    this.fail(new InvalidRequestError("invalid json-rpc envelope"));
  }

  private async dispatchRequest(request: RequestFrame): Promise<void> {
    const handler = this.handlers.get(request.method.trim());
    if (!handler) {
      await this.sendError(request.id as JSONRPCID, new MethodNotFoundError(request.method));
      return;
    }

    try {
      const result = await handler(request.params, request);
      await this.sendResult(request.id as JSONRPCID, result ?? null);
    } catch (error) {
      await this.sendError(request.id as JSONRPCID, ensureRPCError(error));
    }
  }

  private dispatchResponse(response: ResponseFrame): void {
    const pending = this.pending.get(keyOfID(response.id));
    if (!pending) {
      return;
    }
    this.pending.delete(keyOfID(response.id));

    if (response.error) {
      pending.reject(errorFromObject(response.error));
      return;
    }

    pending.resolve(response.result);
  }

  private async sendResult(id: JSONRPCID, result: unknown): Promise<void> {
    this.writeFrame({
      jsonrpc: JSON_RPC_VERSION,
      id,
      result,
    } satisfies ResponseFrame);
  }

  private async sendError(id: JSONRPCID, error: Error): Promise<void> {
    const rpcError = ensureRPCError(error);
    this.writeFrame({
      jsonrpc: JSON_RPC_VERSION,
      id,
      error: rpcError.toJSONRPC(),
    } satisfies ResponseFrame);
  }

  private writeFrame(frame: RequestFrame | ResponseFrame): void {
    const encoded = JSON.stringify(frame);
    if (Buffer.byteLength(encoded) > this.maxMessageBytes) {
      throw new Error(`message exceeds ${this.maxMessageBytes} bytes`);
    }
    this.output.write(`${encoded}\n`);
  }

  private fail(error: Error): void {
    if (this.closed) {
      return;
    }
    this.closed = true;

    this.input.off("data", this.onDataBound);
    this.input.off("error", this.onErrorBound);
    this.input.off("end", this.onEndBound);
    this.input.off("close", this.onEndBound);

    for (const pending of this.pending.values()) {
      pending.reject(error);
    }
    this.pending.clear();

    for (const listener of this.errorListeners) {
      listener(error);
    }
  }
}

export class NotReadyTransport implements TransportLike {
  public handle(): void {}
  public onTransportError(): () => void {
    return () => {};
  }
  public start(): void {}
  public async close(): Promise<void> {}
  public async call(_method?: string, _params?: unknown): Promise<never> {
    throw new NotInitializedError();
  }
}
