import type { IncomingHttpHeaders } from "node:http";

/**
 * A write must come from the gallery page itself. Browsers label cross-origin requests with
 * `Origin` and `Sec-Fetch-Site`, even the "simple" `text/plain` or multipart posts that skip CORS
 * preflight; the CLI and other non-browser callers send neither and stay allowed.
 */
export function isTrustedWrite(headers: IncomingHttpHeaders) {
  const site = headers["sec-fetch-site"];
  if (site !== undefined && site !== "same-origin" && site !== "none") return false;
  const origin = headers.origin;
  if (origin === undefined) return true;
  try {
    return new URL(origin).host === headers.host;
  } catch {
    return false;
  }
}
