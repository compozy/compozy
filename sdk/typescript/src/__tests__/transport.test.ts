import { PassThrough } from "node:stream";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  CapabilityDeniedError,
  InvalidRequestError,
  NotInitializedError,
  ParseError,
} from "../errors.js";
import { JSON_RPC_CANCEL_METHOD } from "../index.js";
import { DEFAULT_MAX_MESSAGE_BYTES, NotReadyTransport, StdioTransport } from "../transport.js";
import { createMockTransportPair } from "../testing/mock-transport.js";

function createTransport() {
  const input = new PassThrough();
  const output = new PassThrough();
  const transport = new StdioTransport({ input, output });
  const frames: string[] = [];
  output.on("data", (chunk: Buffer | string) => {
    frames.push(
      ...String(chunk)
        .split("\n")
        .map(line => line.trim())
        .filter(Boolean)
    );
  });
  return { input, output, transport, frames };
}

async function waitFor(predicate: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    if (predicate()) {
      return;
    }
    await new Promise(resolve => setTimeout(resolve, 5));
  }
  throw new Error("condition not met");
}

describe("StdioTransport", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("encodes one JSON object per line", async () => {
    const { input, transport, frames } = createTransport();

    const pending = transport.call("sessions/list", {});
    await waitFor(() => frames.length === 1);

    expect(frames).toHaveLength(1);
    expect(JSON.parse(frames[0]!)).toMatchObject({
      jsonrpc: "2.0",
      id: 1,
      method: "sessions/list",
      params: {},
    });

    input.write(`${JSON.stringify({ jsonrpc: "2.0", id: 1, result: [] })}\n`);
    await expect(pending).resolves.toEqual([]);
  });

  it("decodes multiple concurrent requests correctly", async () => {
    const { input, frames, transport } = createTransport();

    transport.handle("fast", async () => ({ method: "fast" }));
    transport.handle("slow", async () => {
      await new Promise(resolve => setTimeout(resolve, 10));
      return { method: "slow" };
    });
    transport.start();

    input.write(`${JSON.stringify({ jsonrpc: "2.0", id: 1, method: "slow", params: {} })}\n`);
    input.write(`${JSON.stringify({ jsonrpc: "2.0", id: 2, method: "fast", params: {} })}\n`);

    await waitFor(() => frames.length === 2);
    const responses = frames.map(frame => JSON.parse(frame));

    expect(responses).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: 1, result: { method: "slow" } }),
        expect.objectContaining({ id: 2, result: { method: "fast" } }),
      ])
    );
  });

  it("sends cancellation and discards a late outbound response", async () => {
    const { input, frames, transport } = createTransport();
    const controller = new AbortController();
    const pending = transport.call("view/event", {}, controller.signal);
    await waitFor(() => frames.length === 1);

    controller.abort(new DOMException("superseded", "AbortError"));
    await expect(pending).rejects.toMatchObject({ name: "AbortError", message: "superseded" });
    await waitFor(() => frames.length === 2);
    expect(JSON.parse(frames[1]!)).toEqual({
      jsonrpc: "2.0",
      method: JSON_RPC_CANCEL_METHOD,
      params: { id: 1 },
    });

    input.write(`${JSON.stringify({ jsonrpc: "2.0", id: 1, result: { stale: true } })}\n`);
    await new Promise<void>(resolve => {
      setImmediate(resolve);
    });
    expect(frames).toHaveLength(2);
  });

  it("aborts an inbound handler and omits its late response", async () => {
    const { input, frames, transport } = createTransport();
    let signal: AbortSignal | undefined;
    let release: (() => void) | undefined;
    let handlerReturned = false;
    transport.handle("view/event", async (_params, _request, requestSignal) => {
      signal = requestSignal;
      await new Promise<void>(resolve => {
        release = resolve;
      });
      handlerReturned = true;
      return { stale: true };
    });
    transport.start();
    input.write(`${JSON.stringify({ jsonrpc: "2.0", id: 7, method: "view/event" })}\n`);
    await waitFor(() => signal !== undefined);

    input.write(
      `${JSON.stringify({
        jsonrpc: "2.0",
        method: JSON_RPC_CANCEL_METHOD,
        params: { id: 7 },
      })}\n`
    );
    await waitFor(() => signal?.aborted === true);
    release?.();
    await waitFor(() => handlerReturned);
    await new Promise<void>(resolve => {
      setImmediate(resolve);
    });

    expect(frames).toHaveLength(0);
  });

  it("rejects messages over 10 MiB", async () => {
    const { transport } = createTransport();

    await expect(
      transport.call("memory/store", { blob: "a".repeat(DEFAULT_MAX_MESSAGE_BYTES) })
    ).rejects.toThrow(`message exceeds ${DEFAULT_MAX_MESSAGE_BYTES} bytes`);
  });

  it("ignores notifications with no id field", async () => {
    const { input, frames, transport } = createTransport();
    const handler = vi.fn(async () => ({ ok: true }));

    transport.handle("health_check", handler);
    transport.start();
    input.write(`${JSON.stringify({ jsonrpc: "2.0", method: "health_check", params: {} })}\n`);

    await new Promise(resolve => setTimeout(resolve, 25));

    expect(handler).not.toHaveBeenCalled();
    expect(frames).toHaveLength(0);
  });

  it("returns method not found for unknown inbound methods", async () => {
    const { input, frames, transport } = createTransport();

    transport.start();
    input.write(`${JSON.stringify({ jsonrpc: "2.0", id: 1, method: "missing", params: {} })}\n`);

    await waitFor(() => frames.length === 1);
    expect(JSON.parse(frames[0]!)).toMatchObject({
      id: 1,
      error: {
        code: -32601,
        message: "Method not found",
      },
    });
  });

  it("serializes typed handler errors", async () => {
    const { input, frames, transport } = createTransport();

    transport.handle("sessions/create", async () => {
      throw new CapabilityDeniedError({
        method: "sessions/create",
        required: ["session.write"],
        granted: ["session.read"],
      });
    });
    transport.start();
    input.write(
      `${JSON.stringify({ jsonrpc: "2.0", id: 1, method: "sessions/create", params: {} })}\n`
    );

    await waitFor(() => frames.length === 1);
    expect(JSON.parse(frames[0]!)).toMatchObject({
      id: 1,
      error: {
        code: -32001,
        message: "Capability denied",
        data: {
          method: "sessions/create",
        },
      },
    });
  });

  it("rejects pending requests when the transport closes", async () => {
    const { input, transport } = createTransport();
    const pending = transport.call("sessions/list", {});

    input.end();
    await expect(pending).rejects.toThrow("transport closed");
  });

  it("emits parse errors for invalid json frames", async () => {
    const { input, transport } = createTransport();
    const listener = vi.fn();

    transport.onTransportError(listener);
    transport.start();
    input.write("{invalid json}\n");

    await waitFor(() => listener.mock.calls.length === 1);
    expect(listener.mock.calls[0]![0]).toBeInstanceOf(ParseError);
  });

  it("emits invalid request errors for batch envelopes", async () => {
    const { input, transport } = createTransport();
    const listener = vi.fn();

    transport.onTransportError(listener);
    transport.start();
    input.write('[{"jsonrpc":"2.0"}]\n');

    await waitFor(() => listener.mock.calls.length === 1);
    expect(listener.mock.calls[0]![0]).toBeInstanceOf(InvalidRequestError);
  });

  it("provides a not-ready transport for guarded host calls", async () => {
    const transport = new NotReadyTransport();

    transport.start();
    await expect(transport.call("sessions/list")).rejects.toBeInstanceOf(NotInitializedError);
    await transport.close();
  });

  it("suppresses a late mock-transport result after abort", async () => {
    const pair = createMockTransportPair();
    let started!: () => void;
    const startedPromise = new Promise<void>(resolve => {
      started = resolve;
    });
    let release!: () => void;
    const held = new Promise<void>(resolve => {
      release = resolve;
    });
    pair.extension.handle("slow", async () => {
      started();
      await held;
      return { stale: true };
    });

    const controller = new AbortController();
    const pending = pair.host.call("slow", {}, controller.signal);
    await startedPromise;
    controller.abort(new DOMException("superseded", "AbortError"));
    await expect(pending).rejects.toMatchObject({ name: "AbortError", message: "superseded" });
    release();
    await new Promise<void>(resolve => {
      setImmediate(resolve);
    });
  });
});
