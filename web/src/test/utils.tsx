import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement, ReactNode } from "react";
import { act } from "react";
import { vi } from "vitest";

export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}

export function withQuery(queryClient: QueryClient) {
  return function Provider({ children }: { children: ReactNode }): ReactElement {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

export interface FetchStubResponse {
  status: number;
  body?: unknown;
  matcher: (input: RequestInfo | URL, init?: RequestInit) => boolean;
}

interface RecordedCall {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: string | null;
}

function resolveMethod(input: RequestInfo | URL, init?: RequestInit): string {
  if (input instanceof Request) {
    return input.method.toUpperCase();
  }
  return (init?.method ?? "GET").toUpperCase();
}

function resolveUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return input;
  }
  if (input instanceof URL) {
    return input.toString();
  }
  return input.url;
}

async function extractBody(input: RequestInfo | URL, init?: RequestInit): Promise<string | null> {
  if (init?.body) {
    if (typeof init.body === "string") {
      return init.body;
    }
    if (init.body instanceof ArrayBuffer) {
      return new TextDecoder().decode(init.body);
    }
  }
  if (input instanceof Request) {
    try {
      const cloned = input.clone();
      const text = await cloned.text();
      return text.length > 0 ? text : null;
    } catch {
      return null;
    }
  }
  return null;
}

function collectHeaders(input: RequestInfo | URL, init?: RequestInit): Record<string, string> {
  const headers: Record<string, string> = {};
  if (input instanceof Request) {
    input.headers.forEach((value, key) => {
      headers[key.toLowerCase()] = value;
    });
  }
  const rawHeaders = init?.headers;
  if (rawHeaders instanceof Headers) {
    rawHeaders.forEach((value, key) => {
      headers[key.toLowerCase()] = value;
    });
  } else if (Array.isArray(rawHeaders)) {
    for (const entry of rawHeaders) {
      if (Array.isArray(entry) && entry.length === 2) {
        headers[entry[0].toLowerCase()] = entry[1];
      }
    }
  } else if (rawHeaders && typeof rawHeaders === "object") {
    for (const [key, value] of Object.entries(rawHeaders)) {
      if (typeof value === "string") {
        headers[key.toLowerCase()] = value;
      }
    }
  }
  return headers;
}

export function installFetchStub(responses: FetchStubResponse[]) {
  const calls: RecordedCall[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = resolveUrl(input);
    const method = resolveMethod(input, init);
    const headers = collectHeaders(input, init);
    const body = await extractBody(input, init);
    calls.push({ url, method, headers, body });
    const match = responses.find(candidate => candidate.matcher(input, init));
    if (!match) {
      throw new Error(`unexpected fetch ${method} ${url}`);
    }
    const payload = match.body;
    return new Response(payload === undefined ? null : JSON.stringify(payload), {
      status: match.status,
      headers: { "content-type": "application/json" },
    });
  });
  const previous = globalThis.fetch;
  globalThis.fetch = fetchMock as unknown as typeof globalThis.fetch;
  return {
    fetchMock,
    calls,
    restore: () => {
      globalThis.fetch = previous;
    },
  };
}

export function matchPath(path: string, method: string = "GET") {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const url = resolveUrl(input);
    const actual = resolveMethod(input, init);
    return url.endsWith(path) && actual === method.toUpperCase();
  };
}

export async function flushAsync(times: number = 2): Promise<void> {
  for (let i = 0; i < times; i += 1) {
    // eslint-disable-next-line @typescript-eslint/await-thenable
    await act(async () => {
      await Promise.resolve();
    });
  }
}
