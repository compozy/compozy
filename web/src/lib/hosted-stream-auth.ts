import { apiClient } from "./api-client";

class HostedStreamAuthError extends Error {
  constructor(
    message: string,
    readonly status: number
  ) {
    super(message);
    this.name = "HostedStreamAuthError";
  }
}

const HOSTED_TICKET_QUERY_PARAM = "remote_ticket";

/**
 * Mints a single-use hosted browser stream ticket bound to one stream path
 * (`POST /remote/v1/stream-tickets`). Passed as `?remote_ticket=` so it never
 * collides with terminal `?ticket=` attach passes.
 */
export async function acquireHostedStreamTicket(
  targetUrl: string,
  signal?: AbortSignal
): Promise<string> {
  const baseOrigin = typeof window === "undefined" ? "http://localhost" : window.location.origin;
  const path = new URL(targetUrl, baseOrigin).pathname;

  const { data, error, response } = await apiClient.POST("/remote/v1/stream-tickets", {
    params: { header: { DPoP: "" } },
    body: { path },
    signal,
  });

  if (!response.ok || error !== undefined) {
    throw new HostedStreamAuthError(
      `Failed to authorize the hosted live stream: ${response.status}`,
      response.status
    );
  }

  const rawTicket = data?.ticket;
  if (typeof rawTicket !== "string" || rawTicket.trim() === "") {
    throw new HostedStreamAuthError(
      `Failed to authorize the hosted live stream: invalid ticket (${response.status})`,
      response.status
    );
  }

  return rawTicket.trim();
}

/** Appends a hosted `?remote_ticket=` to a relative or absolute URL without disturbing other params. */
export function appendHostedStreamTicket(url: string, ticket: string): string {
  return appendNamedStreamTicket(url, HOSTED_TICKET_QUERY_PARAM, ticket);
}

function appendNamedStreamTicket(url: string, param: string, ticket: string): string {
  const [beforeHash, hash] = splitHash(url);
  const separator = beforeHash.includes("?") ? "&" : "?";
  const authorized = `${beforeHash}${separator}${param}=${encodeURIComponent(ticket)}`;
  return hash === undefined ? authorized : `${authorized}#${hash}`;
}

function splitHash(url: string): [string, string | undefined] {
  const index = url.indexOf("#");
  return index === -1 ? [url, undefined] : [url.slice(0, index), url.slice(index + 1)];
}
