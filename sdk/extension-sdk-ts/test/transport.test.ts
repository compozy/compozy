import { PassThrough } from "node:stream";

import { describe, expect, it } from "vitest";

import {
  EOFError,
  RPCError,
  StdIOTransport,
  isRPCError,
  newInternalError,
  newInvalidParamsError,
  newInvalidRequestError,
  newMethodNotFoundError,
  newParseError,
  normalizeMessageID,
} from "../src/transport.js";
import { MAX_MESSAGE_SIZE } from "../src/types.js";

describe("StdIOTransport", () => {
  it("encodes one request and decodes one response", async () => {
    const input = new PassThrough();
    const output = new PassThrough();
    const transport = new StdIOTransport(input, output);

    await transport.writeMessage({
      id: 7,
      method: "host.tasks.create",
      params: { workflow: "demo" },
    });

    const encoded = output.read()?.toString("utf8");
    expect(encoded).toContain('"jsonrpc":"2.0"');
    expect(encoded).toContain('"id":7');
    expect(encoded).toContain('"method":"host.tasks.create"');

    input.write('{"jsonrpc":"2.0","id":7,"result":{"ok":true}}\n');
    await expect(transport.readMessage()).resolves.toEqual({
      jsonrpc: "2.0",
      id: 7,
      result: { ok: true },
    });
  });

  it("rejects frames larger than 10 MiB", async () => {
    const input = new PassThrough();
    const output = new PassThrough();
    const transport = new StdIOTransport(input, output);

    const oversized = "x".repeat(MAX_MESSAGE_SIZE + 1);
    input.write(`${oversized}\n`);

    await expect(transport.readMessage()).rejects.toMatchObject<RPCError>({
      code: -32603,
      message: "Internal error",
      data: { reason: "message_too_large" },
    });
  });

  it("queues parsed messages and ignores blank lines", async () => {
    const input = new PassThrough();
    const output = new PassThrough();
    const transport = new StdIOTransport(input, output);

    input.write("\n");
    input.write('{"jsonrpc":"2.0","id":"1","result":{"ok":true}}\n');
    input.write('{"jsonrpc":"2.0","id":"2","result":{"ok":false}}\n');

    await expect(transport.readMessage()).resolves.toMatchObject({ id: "1", result: { ok: true } });
    await expect(transport.readMessage()).resolves.toMatchObject({
      id: "2",
      result: { ok: false },
    });
    await transport.close();
    expect(output.read()).toBeNull();
  });

  it("converts invalid JSON into a structured parse error", async () => {
    const input = new PassThrough();
    const output = new PassThrough();
    const transport = new StdIOTransport(input, output);

    input.write("{not-json}\n");

    await expect(transport.readMessage()).rejects.toMatchObject<RPCError>({
      code: -32700,
      message: "Parse error",
    });
  });

  it("rejects pending readers with EOF on close", async () => {
    const input = new PassThrough();
    const output = new PassThrough();
    const transport = new StdIOTransport(input, output);

    const pending = transport.readMessage();
    await transport.close();

    await expect(pending).rejects.toBeInstanceOf(EOFError);
    await expect(transport.writeMessage({ id: 1, result: {} })).rejects.toBeInstanceOf(EOFError);
  });

  it("rejects oversized outbound frames and exposes helper constructors", async () => {
    const input = new PassThrough();
    const output = new PassThrough();
    const transport = new StdIOTransport(input, output);

    await expect(
      transport.writeMessage({
        id: 9,
        result: { body: "x".repeat(MAX_MESSAGE_SIZE) },
      })
    ).rejects.toMatchObject<RPCError>({
      code: -32603,
      data: { reason: "message_too_large" },
    });

    const parse = newParseError({ reason: "bad_json" });
    const invalidRequest = newInvalidRequestError({ reason: "bad_request" });
    const invalidParams = newInvalidParamsError({ reason: "bad_params" });
    const methodNotFound = newMethodNotFoundError("host.unknown");
    const internal = newInternalError({ reason: "boom" });

    expect(isRPCError(parse)).toBe(true);
    expect(parse.decodeData<{ reason: string }>()).toEqual({ reason: "bad_json" });
    expect(invalidRequest.toShape()).toEqual({
      code: -32600,
      message: "Invalid request",
      data: { reason: "bad_request" },
    });
    expect(invalidParams.toShape()).toEqual({
      code: -32602,
      message: "Invalid params",
      data: { reason: "bad_params" },
    });
    expect(methodNotFound.toShape()).toEqual({
      code: -32601,
      message: "Method not found",
      data: { method: "host.unknown" },
    });
    expect(internal.toShape()).toEqual({
      code: -32603,
      message: "Internal error",
      data: { reason: "boom" },
    });
    expect(normalizeMessageID(17)).toBe("17");
  });
});
