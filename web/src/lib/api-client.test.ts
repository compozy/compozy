import { describe, expect, it } from "vitest";

import {
  ApiRequestError,
  apiErrorMessage,
  isStaleWorkspaceError,
  requireData,
  toTransportError,
} from "./api-client";

describe("toTransportError", () => {
  it("Should parse a well-shaped transport error", () => {
    const error = {
      code: "workspace_context_stale",
      message: "active workspace context is stale",
      request_id: "req-1",
      details: { workspace: "ws-99" },
    };
    expect(toTransportError(error)).toEqual(error);
  });

  it("Should ignore malformed values", () => {
    expect(toTransportError(null)).toBeUndefined();
    expect(toTransportError({ code: 1, message: 2 })).toBeUndefined();
  });
});

describe("isStaleWorkspaceError", () => {
  it("Should match the workspace_context_stale code", () => {
    expect(isStaleWorkspaceError({ code: "workspace_context_stale", message: "stale" })).toBe(true);
  });

  it("Should ignore other codes", () => {
    expect(isStaleWorkspaceError({ code: "forbidden", message: "no" })).toBe(false);
  });
});

describe("apiErrorMessage", () => {
  it("Should prefer transport error messages", () => {
    expect(apiErrorMessage({ code: "x", message: "broke" }, "fallback")).toBe("broke");
  });

  it("Should fall back when the error has no message", () => {
    expect(apiErrorMessage(undefined, "fallback")).toBe("fallback");
  });
});

describe("requireData", () => {
  it("Should return data when present", () => {
    const response = new Response(null, { status: 200 });
    expect(requireData({ ok: true }, response, "fallback", undefined)).toEqual({ ok: true });
  });

  it("Should throw when the response failed and no data was parsed", () => {
    const response = new Response(null, { status: 500 });
    const thrown = captureError(() =>
      requireData(undefined, response, "boom", { code: "x", message: "server said so" })
    );
    expect(thrown).toBeInstanceOf(ApiRequestError);
    const apiError = thrown as ApiRequestError;
    expect(apiError.status).toBe(500);
    expect(apiError.message).toBe("server said so");
  });
});

function captureError(fn: () => unknown): unknown {
  try {
    fn();
    return undefined;
  } catch (error) {
    return error;
  }
}
