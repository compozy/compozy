import { describe, expect, it } from "vitest";

import {
  CapabilityDeniedError,
  InternalError,
  InvalidParamsError,
  MethodNotFoundError,
  NotInitializedError,
  RateLimitedError,
  RPCError,
  ShutdownInProgressError,
  ensureRPCError,
  errorFromObject,
  isRPCError,
} from "../errors.js";

describe("errors", () => {
  it("serializes rpc errors with and without data", () => {
    expect(new RPCError(-1, "boom").toJSONRPC()).toEqual({
      code: -1,
      message: "boom",
    });
    expect(new CapabilityDeniedError({ method: "sessions/create" }).toJSONRPC()).toEqual({
      code: -32001,
      message: "Capability denied",
      data: { method: "sessions/create" },
    });
  });

  it("maps known json-rpc error objects to typed errors", () => {
    expect(
      errorFromObject({
        code: -32601,
        message: "Method not found",
        data: { method: "sessions/list" },
      })
    ).toBeInstanceOf(MethodNotFoundError);
    expect(
      errorFromObject({
        code: -32602,
        message: "Invalid params",
        data: { error: "bad payload" },
      })
    ).toBeInstanceOf(InvalidParamsError);
    expect(
      errorFromObject({
        code: -32001,
        message: "Capability denied",
        data: { method: "sessions/create" },
      })
    ).toBeInstanceOf(CapabilityDeniedError);
    expect(
      errorFromObject({
        code: -32002,
        message: "Rate limited",
        data: { retry_after_ms: 500 },
      })
    ).toBeInstanceOf(RateLimitedError);
    expect(
      errorFromObject({
        code: -32003,
        message: "Not initialized",
        data: { allowed_methods: ["initialize"] },
      })
    ).toBeInstanceOf(NotInitializedError);
    expect(
      errorFromObject({
        code: -32004,
        message: "Shutdown in progress",
        data: { deadline_ms: 1000 },
      })
    ).toBeInstanceOf(ShutdownInProgressError);
    expect(errorFromObject({ code: -32099, message: "Custom" })).toBeInstanceOf(RPCError);
  });

  it("normalizes unknown errors", () => {
    const original = new RateLimitedError({ retry_after_ms: 1000 });
    expect(ensureRPCError(original)).toBe(original);
    expect(ensureRPCError(new Error("explode"))).toBeInstanceOf(InternalError);
    expect(ensureRPCError("explode")).toBeInstanceOf(InternalError);
    expect(isRPCError(original)).toBe(true);
    expect(isRPCError(new Error("nope"))).toBe(false);
  });
});
