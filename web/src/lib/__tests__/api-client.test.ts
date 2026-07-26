import { afterEach, describe, expect, it, vi } from "vitest";

import {
  apiBaseUrl,
  apiClient,
  apiErrorMessage,
  apiRequestFailed,
  daemonApiClient,
  defaultApiErrorMessage,
  requireResponseData,
  runtimeFetch,
} from "@/lib/api-client";

function requestUrl(input: RequestInfo | URL): string {
  if (input instanceof Request) {
    return input.url;
  }
  if (input instanceof URL) {
    return input.toString();
  }
  return input;
}

describe("api client", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("routes daemon browser requests through the runtime fetch and base url", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          daemon: {
            name: "Compozy Daemon",
            status: "ready",
            version: "0.1.0",
            started_at: "2026-04-20T00:00:00Z",
            uptime_seconds: 12,
            http: {
              host: "127.0.0.1",
              port: 2123,
              origin: "http://127.0.0.1:2123",
            },
            workspace_count: 1,
            run_count: 2,
          },
        }),
        {
          status: 200,
          headers: {
            "Content-Type": "application/json",
          },
        }
      )
    );

    vi.stubGlobal("fetch", fetchMock);

    const result = await daemonApiClient.GET("/api/status");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestUrl(fetchMock.mock.calls[0]?.[0] as RequestInfo | URL)).toBe(
      `${apiBaseUrl}/api/status`
    );
    expect(result.response.ok).toBe(true);
    expect(result.data?.daemon.name).toBe("Compozy Daemon");
  });

  it("keeps the existing agh client bound to runtime fetch after module import", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          daemon: {
            status: "ready",
            pid: 42,
            started_at: "2026-04-20T00:00:00Z",
            socket: "/tmp/agh.sock",
            http_host: "127.0.0.1",
            http_port: 2123,
            user_home_dir: "/tmp/home",
            active_sessions: 0,
            total_sessions: 0,
            version: "1.0.0",
          },
        }),
        {
          status: 200,
          headers: {
            "Content-Type": "application/json",
          },
        }
      )
    );

    vi.stubGlobal("fetch", fetchMock);

    const result = await apiClient.GET("/api/status");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestUrl(fetchMock.mock.calls[0]?.[0] as RequestInfo | URL)).toBe(
      `${apiBaseUrl}/api/status`
    );
    expect(result.response.ok).toBe(true);
    expect(result.data?.daemon.pid).toBe(42);
  });

  it("delegates runtimeFetch to the latest global fetch implementation", async () => {
    const response = new Response("ok", { status: 200 });
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);

    const result = await runtimeFetch("http://example.test/api", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ ok: true }),
    });

    expect(fetchMock).toHaveBeenCalledWith("http://example.test/api", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ ok: true }),
    });
    expect(result).toBe(response);
  });
});

describe("api client helpers", () => {
  it("normalizes api error messages from strings and payload objects", () => {
    expect(apiErrorMessage("  daemon offline  ")).toBe("daemon offline");
    expect(apiErrorMessage({ error: "  typed payload  " })).toBe("typed payload");
  });

  it("ignores empty, missing, and non-string api error messages", () => {
    expect(apiErrorMessage("   ")).toBeUndefined();
    expect(apiErrorMessage({ error: "   " })).toBeUndefined();
    expect(apiErrorMessage({ error: 42 })).toBeUndefined();
    expect(apiErrorMessage({})).toBeUndefined();
    expect(apiErrorMessage(null)).toBeUndefined();
  });

  it("prefers a payload message over the fallback response status", () => {
    const response = new Response(null, { status: 422, statusText: "Unprocessable Entity" });

    expect(
      defaultApiErrorMessage("Resolve workspace failed", response, { error: "Bad slug" })
    ).toBe("Bad slug");
  });

  it("builds a fallback api error message when no payload message is available", () => {
    const response = new Response(null, { status: 503, statusText: "Service Unavailable" });

    expect(defaultApiErrorMessage("Resolve workspace failed", response, { reason: "nope" })).toBe(
      "Resolve workspace failed: 503"
    );
  });

  it("treats non-ok responses or returned errors as request failures", () => {
    expect(apiRequestFailed(new Response(null, { status: 200 }), undefined)).toBe(false);
    expect(apiRequestFailed(new Response(null, { status: 500 }), undefined)).toBe(true);
    expect(apiRequestFailed(new Response(null, { status: 200 }), { error: "boom" })).toBe(true);
  });

  it("requires response data for successful typed reads", () => {
    const response = new Response(null, { status: 200 });

    expect(requireResponseData({ ok: true }, response, "Missing payload")).toEqual({ ok: true });
    expect(() => requireResponseData(undefined, response, "Missing payload")).toThrowError(
      "Missing payload: empty response (200)"
    );
  });
});
