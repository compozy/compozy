import type { Middleware } from "openapi-fetch";

export const CSRF_COOKIE_NAME = "compozy_csrf";
export const CSRF_HEADER_NAME = "X-Compozy-CSRF-Token";

const MUTATING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

export function readCsrfToken(source: string | undefined = readDocumentCookie()): string | null {
  if (!source) {
    return null;
  }
  const parts = source.split(";");
  for (const raw of parts) {
    const [rawName, ...rest] = raw.split("=");
    if (!rawName) {
      continue;
    }
    if (rawName.trim() !== CSRF_COOKIE_NAME) {
      continue;
    }
    const value = rest.join("=").trim();
    return value.length > 0 ? decodeURIComponent(value) : null;
  }
  return null;
}

function readDocumentCookie(): string | undefined {
  if (typeof document === "undefined") {
    return undefined;
  }
  return document.cookie;
}

export const csrfMiddleware: Middleware = {
  async onRequest({ request }) {
    const method = request.method.toUpperCase();
    if (!MUTATING_METHODS.has(method)) {
      return request;
    }
    const token = readCsrfToken();
    if (!token) {
      return request;
    }
    if (!request.headers.has(CSRF_HEADER_NAME)) {
      request.headers.set(CSRF_HEADER_NAME, token);
    }
    return request;
  },
};
