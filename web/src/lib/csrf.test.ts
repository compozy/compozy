import { afterEach, describe, expect, it } from "vitest";

import { csrfMiddleware, CSRF_COOKIE_NAME, CSRF_HEADER_NAME, readCsrfToken } from "./csrf";

afterEach(() => {
  document.cookie = `${CSRF_COOKIE_NAME}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
});

describe("readCsrfToken", () => {
  it("Should return null when the csrf cookie is absent", () => {
    expect(readCsrfToken("other=1; another=value")).toBeNull();
  });

  it("Should return the decoded csrf cookie when present", () => {
    const raw = `other=1; ${CSRF_COOKIE_NAME}=abc%20def`;
    expect(readCsrfToken(raw)).toBe("abc def");
  });

  it("Should fall back to document.cookie when no source is provided", () => {
    document.cookie = `${CSRF_COOKIE_NAME}=token-from-doc; path=/`;
    expect(readCsrfToken()).toBe("token-from-doc");
  });
});

describe("csrfMiddleware", () => {
  const onRequest = csrfMiddleware.onRequest as NonNullable<typeof csrfMiddleware.onRequest>;

  it("Should leave GET requests untouched", async () => {
    document.cookie = `${CSRF_COOKIE_NAME}=token; path=/`;
    const request = new Request("http://localhost/api/tasks", { method: "GET" });
    const result = await onRequest({ request } as Parameters<typeof onRequest>[0]);
    const resolved = result instanceof Request ? result : request;
    expect(resolved.headers.has(CSRF_HEADER_NAME)).toBe(false);
  });

  it("Should add the csrf header on mutating requests when a cookie exists", async () => {
    document.cookie = `${CSRF_COOKIE_NAME}=token; path=/`;
    const request = new Request("http://localhost/api/sync", { method: "POST" });
    const result = await onRequest({ request } as Parameters<typeof onRequest>[0]);
    const resolved = result instanceof Request ? result : request;
    expect(resolved.headers.get(CSRF_HEADER_NAME)).toBe("token");
  });

  it("Should not override an already-set csrf header", async () => {
    document.cookie = `${CSRF_COOKIE_NAME}=token; path=/`;
    const request = new Request("http://localhost/api/sync", {
      method: "POST",
      headers: { [CSRF_HEADER_NAME]: "explicit" },
    });
    const result = await onRequest({ request } as Parameters<typeof onRequest>[0]);
    const resolved = result instanceof Request ? result : request;
    expect(resolved.headers.get(CSRF_HEADER_NAME)).toBe("explicit");
  });

  it("Should skip adding the header when no csrf cookie exists", async () => {
    const request = new Request("http://localhost/api/sync", { method: "POST" });
    const result = await onRequest({ request } as Parameters<typeof onRequest>[0]);
    const resolved = result instanceof Request ? result : request;
    expect(resolved.headers.has(CSRF_HEADER_NAME)).toBe(false);
  });
});
