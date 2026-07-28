import { expect, vi } from "vitest";

type FetchExpectation = {
  body?: unknown;
  callIndex?: number;
  method?: string;
  path: string;
  signal?: AbortSignal;
};

export function mockJsonResponse(body: unknown, init: ResponseInit = {}): void {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  vi.mocked(globalThis.fetch).mockResolvedValue(
    new Response(JSON.stringify(body), {
      status: 200,
      ...init,
      headers,
    })
  );
}

export function mockEmptyResponse(init: ResponseInit = {}): void {
  vi.mocked(globalThis.fetch).mockResolvedValue(
    new Response(null, {
      status: 200,
      ...init,
    })
  );
}

export function fetchRequest(callIndex = 0): Request {
  const call = vi.mocked(globalThis.fetch).mock.calls[callIndex];
  expect(call).toBeDefined();

  const [request] = call;
  expect(request).toBeInstanceOf(Request);
  return request as Request;
}

export async function expectFetchRequest({
  body,
  callIndex = 0,
  method = "GET",
  path,
  signal,
}: FetchExpectation): Promise<Request> {
  const request = fetchRequest(callIndex);
  const url = new URL(request.url);

  expect(`${url.pathname}${url.search}`).toBe(path);
  expect(request.method).toBe(method);

  if (signal !== undefined) {
    expect(request.signal).toBeInstanceOf(AbortSignal);
    expect(request.signal.aborted).toBe(signal.aborted);
  }

  if (body !== undefined) {
    expect(await request.clone().json()).toEqual(body);
  }

  return request;
}
