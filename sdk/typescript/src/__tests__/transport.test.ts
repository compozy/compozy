import { PassThrough } from "node:stream";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  CapabilityDeniedError,
  InvalidRequestError,
  NotInitializedError,
  ParseError,
} from "../errors.js";
import { DEFAULT_MAX_MESSAGE_BYTES, NotReadyTransport, StdioTransport } from "../transport.js";

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
});
