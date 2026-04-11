import { MAX_MESSAGE_SIZE } from "./types.js";

export type MessageID = number | string;

export interface RPCErrorShape {
  code: number;
  message: string;
  data?: unknown;
}

export interface Message {
  jsonrpc?: "2.0";
  id?: MessageID;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: RPCErrorShape;
}

export interface Transport {
  readMessage(): Promise<Message>;
  writeMessage(message: Message): Promise<void>;
  close(): Promise<void>;
}

export class EOFError extends Error {
  constructor(message = "end of stream") {
    super(message);
    this.name = "EOFError";
  }
}

export class RPCError extends Error {
  readonly code: number;
  readonly data?: unknown;

  constructor(code: number, message: string, data?: unknown) {
    super(message);
    this.name = "RPCError";
    this.code = code;
    this.data = data;
  }

  static fromShape(shape: RPCErrorShape): RPCError {
    return new RPCError(shape.code, shape.message, shape.data);
  }

  toShape(): RPCErrorShape {
    return { code: this.code, message: this.message, data: this.data };
  }

  decodeData<T>(): T {
    return this.data as T;
  }
}

type ReadWaiter = {
  reject: (error: unknown) => void;
  resolve: (message: Message) => void;
};

export class StdIOTransport implements Transport {
  readonly input: NodeJS.ReadableStream;
  readonly output: NodeJS.WritableStream;

  private readonly queue: Message[] = [];
  private readonly waiters: ReadWaiter[] = [];
  private buffer = "";
  private closed = false;
  private closedError?: unknown;
  private closePromise?: Promise<void>;
  private readonly onData: (chunk: string | Buffer) => void;
  private readonly onEnd: () => void;
  private readonly onError: (error: Error) => void;

  constructor(
    input: NodeJS.ReadableStream = process.stdin,
    output: NodeJS.WritableStream = process.stdout
  ) {
    this.input = input;
    this.output = output;

    if ("setEncoding" in input && typeof input.setEncoding === "function") {
      input.setEncoding("utf8");
    }

    this.onData = chunk => {
      this.buffer += typeof chunk === "string" ? chunk : chunk.toString("utf8");
      this.drainBuffer();
    };
    this.onEnd = () => {
      this.closed = true;
      this.closedError = new EOFError();
      this.flushWaiters(this.closedError);
    };
    this.onError = error => {
      this.closed = true;
      this.closedError = error;
      this.flushWaiters(error);
    };

    input.on("data", this.onData);
    input.on("end", this.onEnd);
    input.on("error", this.onError);
  }

  async readMessage(): Promise<Message> {
    if (this.queue.length > 0) {
      const message = this.queue.shift();
      if (message === undefined) {
        throw new EOFError();
      }
      return message;
    }

    if (this.closed) {
      throw (this.closedError as Error | undefined) ?? new EOFError();
    }

    return new Promise<Message>((resolve, reject) => {
      this.waiters.push({ resolve, reject });
    });
  }

  async writeMessage(message: Message): Promise<void> {
    if (this.closed) {
      throw new EOFError();
    }

    const envelope = { ...message, jsonrpc: "2.0" as const };
    const encoded = JSON.stringify(envelope);
    if (Buffer.byteLength(encoded, "utf8") > MAX_MESSAGE_SIZE) {
      throw newInternalError({ reason: "message_too_large" });
    }

    await new Promise<void>((resolve, reject) => {
      this.output.write(`${encoded}\n`, error => {
        if (error) {
          reject(error);
          return;
        }
        resolve();
      });
    });
  }

  async close(): Promise<void> {
    if (this.closePromise !== undefined) {
      return this.closePromise;
    }

    this.closed = true;
    this.closePromise = Promise.resolve().then(async () => {
      this.input.off("data", this.onData);
      this.input.off("end", this.onEnd);
      this.input.off("error", this.onError);
      this.closedError ??= new EOFError();
      this.flushWaiters(this.closedError);
    });
    return this.closePromise;
  }

  private drainBuffer(): void {
    while (true) {
      const newlineIndex = this.buffer.indexOf("\n");
      if (newlineIndex === -1) {
        if (Buffer.byteLength(this.buffer, "utf8") > MAX_MESSAGE_SIZE) {
          const error = newInternalError({ reason: "message_too_large" });
          this.closed = true;
          this.closedError = error;
          this.flushWaiters(error);
        }
        return;
      }

      const line = this.buffer.slice(0, newlineIndex);
      this.buffer = this.buffer.slice(newlineIndex + 1);
      if (Buffer.byteLength(line, "utf8") > MAX_MESSAGE_SIZE) {
        const error = newInternalError({ reason: "message_too_large" });
        this.closed = true;
        this.closedError = error;
        this.flushWaiters(error);
        return;
      }

      const trimmed = line.trim();
      if (trimmed.length === 0) {
        continue;
      }

      let message: Message;
      try {
        message = JSON.parse(trimmed) as Message;
      } catch (error) {
        this.closed = true;
        this.closedError = newParseError({ error: stringifyError(error) });
        this.flushWaiters(this.closedError);
        return;
      }

      const waiter = this.waiters.shift();
      if (waiter !== undefined) {
        waiter.resolve(message);
        continue;
      }

      this.queue.push(message);
    }
  }

  private flushWaiters(error: unknown): void {
    while (this.waiters.length > 0) {
      const waiter = this.waiters.shift();
      waiter?.reject(error);
    }
  }
}

export function isRPCError(error: unknown): error is RPCError {
  return error instanceof RPCError;
}

export function newParseError(data?: unknown): RPCError {
  return new RPCError(-32700, "Parse error", data);
}

export function newInvalidRequestError(data?: unknown): RPCError {
  return new RPCError(-32600, "Invalid request", data);
}

export function newMethodNotFoundError(method: string): RPCError {
  return new RPCError(-32601, "Method not found", { method });
}

export function newInvalidParamsError(data?: unknown): RPCError {
  return new RPCError(-32602, "Invalid params", data);
}

export function newInternalError(data?: unknown): RPCError {
  return new RPCError(-32603, "Internal error", data);
}

export function normalizeMessageID(id: MessageID): string {
  return String(id);
}

function stringifyError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}
