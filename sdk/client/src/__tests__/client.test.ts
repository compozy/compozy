import { afterEach, describe, expect, expectTypeOf, it, vi } from "vitest";

import { Compozy } from "../client.js";
import { checkVersion } from "../connect.js";
import {
  CompozyApiError,
  CompozyConfigError,
  CompozyStreamError,
  CompozyUnavailableError,
} from "../errors.js";
import { discover as browserDiscover } from "../browser-discovery.js";
import type { OperationResponse } from "../operation-types.js";
import type {
  CreateSessionData,
  CreateSessionResponse,
  SessionPayload,
} from "../generated/sdk/types.gen.js";
import type { SseOperationId } from "../events.js";
import type { UIMessageChunk } from "../prompt.js";

// Invariants: connection discovery/errors, typed resource calls, prompt stream integrity,
// and bounded resumable SSE. Owner: app client HTTP boundary; canonical suite: this file.
const baseUrl = "http://127.0.0.1:2123";
const path = { workspace_id: "workspace", session_id: "session" };
const promptInput = { workspaceId: "workspace", sessionId: "session", message: "hello" };
const identity = {
  schema_version: "1",
  daemon: { pid: 101, version: "0.0.0-dev", http_host: "127.0.0.1", http_port: 2123 },
};
const accepted = {
  status: "queued",
  delivery: "after_turn",
  message_id: "m1",
  idempotency_key: "id-1",
  replayed: false,
  queue_position: 1,
};

function sse(text: string): Response {
  return new Response(text, {
    headers: { "Content-Type": "text/event-stream", "x-vercel-ai-ui-message-stream": "v1" },
  });
}

async function connected(handler: (request: Request) => Response | Promise<Response>) {
  const requests: Request[] = [];
  const transport: typeof fetch = async (input, init) => {
    const request = new Request(input, init);
    if (request.url.endsWith("/api/status/identity")) return Response.json(identity);
    requests.push(request);
    return handler(request);
  };
  const client = await Compozy.connect({
    baseUrl,
    token: "cpz_app_test",
    clientId: "my-app",
    fetch: transport,
  });
  return { client, requests };
}

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("connect and generated resources", () => {
  it("uses the browser origin without reading Node discovery", async () => {
    expect(await browserDiscover({})).toEqual({ baseUrl: "http://localhost:2123" });
    await expect(browserDiscover({ baseUrl: "localhost:bad" })).rejects.toBeInstanceOf(
      CompozyConfigError
    );
  });

  it("returns authoritative identity and sends auth/client headers on every request", async () => {
    const requests: Request[] = [];
    const fetch: typeof globalThis.fetch = async (input, init) => {
      const request = new Request(input, init);
      requests.push(request);
      return Response.json(request.url.endsWith("/identity") ? identity : { tasks: [] });
    };
    const client = await Compozy.connect({ baseUrl, token: "cpz_app_secret", fetch });
    expect(client.daemon).toMatchObject({
      version: "0.0.0-dev",
      httpHost: "127.0.0.1",
      httpPort: 2123,
    });
    expect(await client.tasks.listTasks({ query: { status: "in_progress" } })).toEqual({
      tasks: [],
    });
    expect(requests).toHaveLength(2);
    for (const request of requests) {
      expect(request.headers.get("Authorization")).toBe("Bearer cpz_app_secret");
      expect(request.headers.get("X-Compozy-Client-ID")).toBe("sdk-ts");
    }
  });

  it("bounds an unreachable identity request and gives a daemon startup hint", async () => {
    vi.useFakeTimers();
    const fetch: typeof globalThis.fetch = async (_input, init) =>
      new Promise((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () =>
          reject(new DOMException("aborted", "AbortError"))
        );
      });
    const result = Compozy.connect({ baseUrl, fetch, timeoutMs: 200 });
    const assertion = expect(result).rejects.toMatchObject({
      name: "CompozyUnavailableError",
      message: expect.stringContaining("compozy daemon start"),
    });
    await vi.advanceTimersByTimeAsync(200);
    await assertion;
  });

  it("compares major.minor once and skips development builds", () => {
    const callback = vi.fn();
    checkVersion("0.5.0", callback, "0.4.0");
    expect(callback).toHaveBeenCalledExactlyOnceWith({ client: "0.4.0", daemon: "0.5.0" });
    checkVersion("0.4.0-beta.3", callback, "0.4.0");
    checkVersion("0.5.0", callback, "0.0.0-dev");
    checkVersion("", callback, "0.4.0");
    expect(callback).toHaveBeenCalledTimes(1);
  });

  it.each([
    [
      404,
      {
        code: "session_not_found",
        error: "Session missing",
        details: { id: "session" },
        current_turn_id: "turn-1",
      },
      "session_not_found",
    ],
    [502, "upstream unavailable", "http_502"],
  ] as const)(
    "maps generated HTTP %i errors without losing daemon fields",
    async (status, body, code) => {
      const { client } = await connected(() =>
        typeof body === "string" ? new Response(body, { status }) : Response.json(body, { status })
      );
      const request = client.sessions.getSession({ path });
      await expect(request).rejects.toBeInstanceOf(CompozyApiError);
      await expect(request).rejects.toMatchObject({
        name: "CompozyApiError",
        status,
        code,
      });
      if (status === 404) {
        await expect(client.sessions.getSession({ path })).rejects.toMatchObject({
          message: "Session missing",
          details: { id: "session" },
          currentTurnId: "turn-1",
        });
      }
    }
  );

  it("maps an unavailable generated request to the public transport error", async () => {
    const { client } = await connected(() => {
      throw new TypeError("connection refused");
    });
    await expect(client.tasks.listTasks()).rejects.toBeInstanceOf(CompozyUnavailableError);
  });

  it("preserves generated operation names, request typing, and OpenAPI response helpers", async () => {
    const { client } = await connected(() => Response.json({ tasks: [] }));
    const taskRequest = client.tasks.listTasks({ query: { status: "in_progress" } });
    expectTypeOf<Awaited<typeof taskRequest>>().toEqualTypeOf<
      OperationResponse<"listTasks", 200>
    >();
    expectTypeOf<CreateSessionResponse["session"]>().toEqualTypeOf<SessionPayload>();
    expectTypeOf<CreateSessionData["body"]>().not.toBeNever();
    expectTypeOf<"streamSession">().toExtend<SseOperationId>();
    expectTypeOf<"sendSessionPrompt">().not.toExtend<SseOperationId>();
    expectTypeOf<
      Extract<keyof typeof client.sessions, "streamSession" | "sendSessionPrompt">
    >().toBeNever();
    expectTypeOf(client.appTokens.createAppToken).toBeFunction();
    await taskRequest;
    const invalidCreate = () => {
      // @ts-expect-error unknown request properties must not pass the generated contract
      return client.sessions.createSession({ body: { nope: 1 } });
    };
    const invalidPathName = () => {
      return client.sessions.getSession({
        // @ts-expect-error path parameters must use their generated names
        path: { workspace_id: "workspace", session: "session" },
      });
    };
    const invalidPathType = () => {
      // @ts-expect-error session identifiers must use the generated string type
      return client.sessions.getSession({ path: { workspace_id: "workspace", session_id: 123 } });
    };
    expectTypeOf(invalidCreate).toBeFunction();
    expectTypeOf(invalidPathName).toBeFunction();
    expectTypeOf(invalidPathType).toBeFunction();
  });
});

describe("prompt", () => {
  // Invariant: both prompt helpers supply complete command identity, stable for a caller's key.
  // Owner: SDK request encoding; canonical suite: this prompt boundary suite.
  it("unwraps accepted JSON and preserves mode, idempotency and turn fence", async () => {
    const { client, requests } = await connected(() =>
      Response.json({ prompt: accepted }, { status: 202 })
    );
    expect(
      await client.prompt({
        ...promptInput,
        mode: "queue",
        idempotencyKey: "id-1",
        expectedTurnId: "turn-1",
      })
    ).toEqual({ kind: "accepted", result: accepted });
    expect(await requests[0]!.json()).toEqual({
      message: "hello",
      message_id: "id-1",
      mode: "queue",
      idempotency_key: "id-1",
      expected_turn_id: "turn-1",
    });
    expect(requests[0]!.headers.get("Authorization")).toBe("Bearer cpz_app_test");
    expect(requests[0]!.headers.get("X-Compozy-Client-ID")).toBe("my-app");
  });

  it("preserves one message identity across retries and both prompt helpers", async () => {
    vi.stubGlobal("crypto", undefined);
    const { client, requests } = await connected(() =>
      Response.json({ prompt: accepted }, { status: 202 })
    );
    const input = { ...promptInput, idempotencyKey: "retry/ação key" };
    await client.prompt(input);
    await client.prompt(input);
    await client.promptResponse(input);
    const bodies = await Promise.all(requests.map(request => request.json()));
    expect(bodies).toEqual([
      { message: "hello", message_id: input.idempotencyKey, idempotency_key: input.idempotencyKey },
      { message: "hello", message_id: input.idempotencyKey, idempotency_key: input.idempotencyKey },
      { message: "hello", message_id: input.idempotencyKey, idempotency_key: input.idempotencyKey },
    ]);
  });

  it("generates separate prompt identities on HTTP origins without subtle or randomUUID", async () => {
    const cryptoApi = globalThis.crypto;
    vi.stubGlobal("crypto", { getRandomValues: cryptoApi.getRandomValues.bind(cryptoApi) });
    const { client, requests } = await connected(() =>
      Response.json({ prompt: accepted }, { status: 202 })
    );
    await client.prompt(promptInput);
    await client.promptResponse(promptInput);
    const bodies = await Promise.all(requests.map(request => request.json()));
    for (const body of bodies) {
      expect(body.idempotency_key).toEqual(expect.any(String));
      expect(body.idempotency_key.length).toBeGreaterThan(0);
      expect(body.message_id).toBe(body.idempotency_key);
    }
    expect(bodies[0].idempotency_key).not.toBe(bodies[1].idempotency_key);
  });

  it.each(["", "   "])(
    "preserves an explicitly invalid key %j for daemon validation",
    async key => {
      vi.stubGlobal("crypto", undefined);
      const { client, requests } = await connected(() =>
        Response.json({ error: "idempotency_key is required" }, { status: 400 })
      );
      const input = { ...promptInput, idempotencyKey: key };
      await expect(client.prompt(input)).rejects.toMatchObject({ status: 400 });
      expect((await client.promptResponse(input)).status).toBe(400);
      for (const request of requests) {
        expect(await request.json()).toMatchObject({ message_id: key, idempotency_key: key });
      }
    }
  );

  it.each(["[DONE]", "finish", "abort"] as const)(
    "yields UI chunks in order and ends at %s",
    async ending => {
      const chunks: UIMessageChunk[] = [
        { type: "start", messageId: "m1" },
        { type: "text-delta", id: "text1", delta: "hello" },
      ];
      if (ending !== "[DONE]") chunks.push({ type: ending });
      const { client, requests } = await connected(() =>
        sse(
          chunks.map(chunk => `data: ${JSON.stringify(chunk)}\n\n`).join("") +
            (ending === "[DONE]" ? `data: ${ending}\n\n` : "")
        )
      );
      const reply = await client.prompt(promptInput);
      expect(reply.kind).toBe("stream");
      if (reply.kind !== "stream") throw new Error("expected stream");
      const received = [];
      for await (const chunk of reply.chunks) received.push(chunk);
      expect(received).toEqual(chunks);
      expect(requests).toHaveLength(1);
    }
  );

  it("exposes a readable stream for AI SDK adapters", async () => {
    const { client } = await connected(() =>
      sse('data: {"type":"start","messageId":"m1"}\n\ndata: [DONE]\n\n')
    );
    const reply = await client.prompt(promptInput);
    if (reply.kind !== "stream") throw new Error("expected stream");
    const reader = reply.toReadableStream().getReader();
    expect(await reader.read()).toEqual({ done: false, value: { type: "start", messageId: "m1" } });
    expect((await reader.read()).done).toBe(true);
  });

  it.each([200, 202, 409])("passes raw status %i and body to a proxy unchanged", async status => {
    const response =
      status === 200
        ? sse('data: {"type":"start"}\n\n')
        : Response.json(
            status === 202 ? { prompt: accepted } : { code: "turn_conflict", error: "Busy" },
            { status }
          );
    const expected = await response.clone().text();
    const { client } = await connected(() => response);
    const result = await client.promptResponse(promptInput);
    expect(result).toBe(response);
    expect(result.status).toBe(status);
    expect(await result.text()).toBe(expected);
    if (status === 200) expect(result.headers.get("x-vercel-ai-ui-message-stream")).toBe("v1");
  });

  it("normalizes a successful JSON 200 accepted result to a 202 proxy response", async () => {
    const { client } = await connected(() => Response.json({ prompt: accepted }));
    const response = await client.promptResponse(promptInput);
    expect(response.status).toBe(202);
    expect(await response.json()).toEqual({ prompt: accepted });
  });

  it("maps prompt HTTP errors exactly like generated methods", async () => {
    const { client } = await connected(() =>
      Response.json(
        { code: "turn_conflict", error: "Busy", current_turn_id: "turn-2" },
        { status: 409 }
      )
    );
    await expect(client.prompt(promptInput)).rejects.toMatchObject({
      name: "CompozyApiError",
      status: 409,
      code: "turn_conflict",
      currentTurnId: "turn-2",
    });
  });

  it("stops reading an aborted prompt without replaying its POST", async () => {
    const canceled = vi.fn();
    const controller = new AbortController();
    const { client, requests } = await connected(
      () =>
        new Response(
          new ReadableStream({
            start(stream) {
              stream.enqueue(
                new TextEncoder().encode(
                  'data: {"type":"text-delta","id":"t1","delta":"hello"}\n\n'
                )
              );
            },
            cancel: canceled,
          }),
          { headers: { "Content-Type": "text/event-stream" } }
        )
    );
    const reply = await client.prompt({ ...promptInput, signal: controller.signal });
    if (reply.kind !== "stream") throw new Error("Expected stream");
    const iterator = reply.chunks[Symbol.asyncIterator]();
    expect((await iterator.next()).value).toMatchObject({ type: "text-delta" });
    controller.abort();
    await expect(iterator.next()).rejects.toBeInstanceOf(CompozyStreamError);
    expect(canceled).toHaveBeenCalledOnce();
    expect(requests).toHaveLength(1);
  });

  it.each([
    'data: {"type":"text-delta"',
    "data: not-json\n\n",
    'data: {"type":"text-delta","id":"t1","delta":"partial"}\n\n',
    "",
  ])("rejects incomplete or malformed prompts without replaying POST", async body => {
    const { client, requests } = await connected(() => sse(body));
    const reply = await client.prompt(promptInput);
    if (reply.kind !== "stream") throw new Error("expected stream");
    await expect(async () => {
      for await (const _chunk of reply.chunks) {
        /* consume */
      }
    }).rejects.toBeInstanceOf(CompozyStreamError);
    expect(requests).toHaveLength(1);
  });
});

describe("events", () => {
  it("preserves event names, ids and degraded payloads and stops when aborted", async () => {
    const controller = new AbortController();
    const frames = [
      { event: "transcript_snapshot", id: "1", data: { sequence: 1 } },
      { event: "transcript_delta", id: "2", data: { sequence: 2 } },
      { event: "stream.consumer_degraded", id: "3", data: { reason: "slow" } },
    ];
    const { client, requests } = await connected(() =>
      sse(
        frames
          .map(
            frame =>
              `event: ${frame.event}\nid: ${frame.id}\ndata: ${JSON.stringify(frame.data)}\n\n`
          )
          .join("")
      )
    );
    const received = [];
    for await (const frame of client.events("streamSession", { path, signal: controller.signal })) {
      received.push(frame);
      if (received.length === 3) controller.abort();
    }
    expect(received).toEqual(frames);
    expect(requests).toHaveLength(1);
    expect(requests[0]!.headers.get("Authorization")).toBe("Bearer cpz_app_test");
    expect(requests[0]!.headers.get("X-Compozy-Client-ID")).toBe("my-app");
  });

  it.each([false, true])(
    "cancels an open response body when the caller aborts (rejected cleanup: %s)",
    async rejectCancellation => {
      const canceled = vi.fn(() =>
        rejectCancellation
          ? Promise.reject(new DOMException("aborted", "AbortError"))
          : Promise.resolve()
      );
      const controller = new AbortController();
      const { client } = await connected(
        () =>
          new Response(
            new ReadableStream({
              start(stream) {
                stream.enqueue(new TextEncoder().encode('id: 1\ndata: {"sequence":1}\n\n'));
              },
              cancel: canceled,
            }),
            { headers: { "Content-Type": "text/event-stream" } }
          )
      );
      const iterator = client
        .events("streamSession", { path, signal: controller.signal })
        [Symbol.asyncIterator]();
      expect((await iterator.next()).value).toMatchObject({ id: "1" });
      controller.abort();
      expect((await iterator.next()).done).toBe(true);
      expect(canceled).toHaveBeenCalledOnce();
    }
  );

  it.each(["streamSession", "streamGlobalSession"] as const)(
    "resumes %s with its latest cursor and fences and bounded backoff",
    async operationId => {
      vi.useFakeTimers();
      const controller = new AbortController();
      const { client, requests } = await connected(() =>
        requests.length === 1
          ? sse(
              'event: transcript_snapshot\nid: 1201\ndata: {"epoch":3,"generation":4,"entries":[],"max_sequence":1201,"reset":true}\n\n'
            )
          : new Response("unavailable", { status: 503 })
      );
      const iterator = client
        .events(operationId, {
          path,
          query: { epoch: 1, generation: 2, limit: 3, profile: "marketing" },
          signal: controller.signal,
        })
        [Symbol.asyncIterator]();
      expect((await iterator.next()).value).toMatchObject({ id: "1201" });
      const next = iterator.next();
      await vi.advanceTimersByTimeAsync(499);
      expect(requests).toHaveLength(1);
      await vi.advanceTimersByTimeAsync(1);
      expect(requests).toHaveLength(2);
      expect(requests[1]!.headers.get("Last-Event-ID")).toBe("1201");
      const resumed = new URL(requests[1]!.url);
      expect(Object.fromEntries(resumed.searchParams)).toMatchObject({
        epoch: "3",
        generation: "4",
        limit: "3",
        profile: "marketing",
      });
      await vi.advanceTimersByTimeAsync(999);
      expect(requests).toHaveLength(2);
      await vi.advanceTimersByTimeAsync(1);
      expect(requests).toHaveLength(3);
      expect(requests[2]!.headers.get("Last-Event-ID")).toBe("1201");
      controller.abort();
      expect((await next).done).toBe(true);
      await vi.runAllTimersAsync();
      expect(requests).toHaveLength(3);
    }
  );

  it.each([
    [401, "app_token_revoked"],
    [404, "session_not_found"],
    [410, "gone"],
  ] as const)("ends on %i without reconnecting", async (status, code) => {
    const { client, requests } = await connected(() =>
      Response.json({ code, error: "Access ended" }, { status })
    );
    const iterator = client.events("streamSession", { path })[Symbol.asyncIterator]();
    await expect(iterator.next()).rejects.toMatchObject({ name: "CompozyApiError", status, code });
    expect(requests).toHaveLength(1);
  });

  it.each([
    [401, "app_token_revoked"],
    [404, "session_not_found"],
  ] as const)("stops after a resumed connection returns %i", async (status, code) => {
    vi.useFakeTimers();
    const { client, requests } = await connected(() =>
      requests.length === 1
        ? sse('id: 9\ndata: {"sequence":9}\n\n')
        : Response.json({ code, error: "Access ended" }, { status })
    );
    const iterator = client.events("streamSession", { path })[Symbol.asyncIterator]();
    expect((await iterator.next()).value).toMatchObject({ id: "9" });
    const failure = expect(iterator.next()).rejects.toMatchObject({
      name: "CompozyApiError",
      status,
      code,
    });
    await vi.advanceTimersByTimeAsync(500);
    await failure;
    expect(requests[1]!.headers.get("Last-Event-ID")).toBe("9");
    await vi.runAllTimersAsync();
    expect(requests).toHaveLength(2);
  });

  it("throws on malformed JSON and names its operation", async () => {
    const { client, requests } = await connected(() => sse("data: not-json\n\n"));
    const iterator = client.events("streamSession", { path })[Symbol.asyncIterator]();
    await expect(iterator.next()).rejects.toMatchObject({
      name: "CompozyStreamError",
      message: expect.stringContaining("streamSession"),
    });
    expect(requests).toHaveLength(1);
  });

  it("exhausts exactly twenty attempts with capped backoff", async () => {
    vi.useFakeTimers();
    const { client, requests } = await connected(() => {
      throw new TypeError("offline");
    });
    const iterator = client.events("streamSession", { path })[Symbol.asyncIterator]();
    const assertion = expect(iterator.next()).rejects.toBeInstanceOf(CompozyStreamError);
    for (let failure = 1; failure < 20; failure++) {
      await vi.advanceTimersByTimeAsync(0);
      expect(requests).toHaveLength(failure);
      await vi.advanceTimersByTimeAsync(Math.min(500 * 2 ** (failure - 1), 8000));
    }
    await assertion;
    expect(requests).toHaveLength(20);
  });

  it("resets the failure budget and delay after a frame", async () => {
    vi.useFakeTimers();
    const { client, requests } = await connected(() => {
      if (requests.length === 20) return sse('id: 1\ndata: {"sequence":1}\n\n');
      throw new TypeError("offline");
    });
    const iterator = client.events("streamSession", { path })[Symbol.asyncIterator]();
    const first = iterator.next();
    for (let failure = 1; failure < 20; failure++) {
      await vi.advanceTimersByTimeAsync(0);
      await vi.advanceTimersByTimeAsync(Math.min(500 * 2 ** (failure - 1), 8000));
    }
    expect((await first).value).toMatchObject({ id: "1" });
    const assertion = expect(iterator.next()).rejects.toBeInstanceOf(CompozyStreamError);
    await vi.advanceTimersByTimeAsync(499);
    expect(requests).toHaveLength(20);
    await vi.advanceTimersByTimeAsync(1);
    expect(requests).toHaveLength(21);
    await vi.runAllTimersAsync();
    await assertion;
    expect(requests).toHaveLength(40);
  });
});
