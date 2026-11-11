import { afterEach, describe, expect, it, vi } from "vitest";

import {
  apiBaseUrl,
  apiClient,
  apiErrorMessage,
  apiRequestFailed,
  daemonFetch,
  defaultApiErrorMessage,
  requireResponseData,
  startTraceAction,
} from "@/lib/api-client";

const TRACEPARENT = /^00-[0-9a-f]{32}-[0-9a-f]{16}-00$/;

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

  it("keeps the existing compozy client bound to runtime fetch after module import", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          daemon: {
            status: "ready",
            pid: 42,
            started_at: "2026-04-20T00:00:00Z",
            socket: "/tmp/compozy.sock",
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
    const clientID = (fetchMock.mock.calls[0]?.[0] as Request | undefined)?.headers.get(
      "X-Compozy-Client-ID"
    );
    expect(clientID).toMatch(/^web-[a-zA-Z0-9_-]{1,76}$/);
    expect(result.response.ok).toBe(true);
    expect(result.data?.daemon.pid).toBe(42);
  });

  it("loads and sends a stable client identifier when randomUUID is unavailable", async () => {
    const getRandomValues = globalThis.crypto.getRandomValues.bind(globalThis.crypto);
    vi.stubGlobal("crypto", { getRandomValues });
    vi.resetModules();
    const { apiClient: client } = await import("@/lib/api-client");
    const fetchMock = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify({}), { headers: { "Content-Type": "application/json" } })
        )
      );
    vi.stubGlobal("fetch", fetchMock);

    await client.GET("/api/status");
    await client.GET("/api/status");

    expect(fetchMock).toHaveBeenCalledTimes(2);
    const first = (fetchMock.mock.calls[0]?.[0] as Request | undefined)?.headers.get(
      "X-Compozy-Client-ID"
    );
    const second = (fetchMock.mock.calls[1]?.[0] as Request | undefined)?.headers.get(
      "X-Compozy-Client-ID"
    );
    expect(first).toMatch(/^web-[a-zA-Z0-9_-]{1,76}$/);
    expect(second).toBe(first);
  });
});

// Invariant: daemon-bound requests join one unsampled W3C trace per user action; other origins
// never receive trace context.
describe("api client trace propagation", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubFetch() {
    const fetchMock = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify({}), { headers: { "Content-Type": "application/json" } })
        )
      );
    vi.stubGlobal("fetch", fetchMock);
    return (call: number) =>
      (fetchMock.mock.calls[call]?.[0] as Request | undefined)?.headers.get("traceparent");
  }

  const traceId = (traceparent: string | null | undefined) => traceparent?.split("-")[1];

  it("Should send an unsampled traceparent to the daemon base URL", async () => {
    const traceparent = stubFetch();

    await apiClient.GET("/api/status");
    await apiClient.GET("/api/status");

    expect(traceparent(0)).toMatch(TRACEPARENT);
    expect(traceparent(1)).toMatch(TRACEPARENT);
    expect(traceId(traceparent(1))).not.toBe(traceId(traceparent(0)));
  });

  it("Should not send a traceparent to another origin", async () => {
    const traceparent = stubFetch();
    const action = startTraceAction();

    await apiClient.GET("/api/status", { baseUrl: "https://other.example" });
    await apiClient.GET("/api/status", { baseUrl: "https://other.example", headers: action() });

    expect(traceparent(0)).toBeNull();
    expect(traceparent(1)).toBeNull();
  });

  it("Should send an unsampled traceparent with daemon requests outside the typed client", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);

    await daemonFetch(`${apiBaseUrl}/api/sessions/s/prompt`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
    await daemonFetch(new Request(`${apiBaseUrl}/api/sessions/s/stop`, { method: "POST" }));
    await daemonFetch("https://other.example/api/status", { headers: { traceparent: "kept-out" } });

    const [prompt, stop, other] = fetchMock.mock.calls as [
      [string, RequestInit],
      [Request],
      [string, RequestInit],
    ];
    const promptHeaders = new Headers(prompt[1].headers);
    expect(promptHeaders.get("traceparent")).toMatch(TRACEPARENT);
    expect(promptHeaders.get("Content-Type")).toBe("application/json");
    expect(stop[0].headers.get("traceparent")).toMatch(TRACEPARENT);
    expect(new Headers(other[1].headers).get("traceparent")).toBeNull();
  });

  it("Should share one trace id across the requests of one action", async () => {
    const traceparent = stubFetch();
    const first = startTraceAction();
    const second = startTraceAction();

    await apiClient.GET("/api/status", { headers: first() });
    await apiClient.GET("/api/status", { headers: first() });
    await apiClient.GET("/api/status", { headers: second() });

    expect(traceparent(0)).toMatch(TRACEPARENT);
    expect(traceparent(1)).toMatch(TRACEPARENT);
    expect(traceparent(1)).not.toBe(traceparent(0));
    expect(traceId(traceparent(1))).toBe(traceId(traceparent(0)));
    expect(traceId(traceparent(2))).not.toBe(traceId(traceparent(0)));
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
