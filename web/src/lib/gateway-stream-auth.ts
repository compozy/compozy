import { apiClient } from "./api-client";

/**
 * Stream authentication for the page's own origin.
 *
 * A tier listener authenticates streams with a single-use ticket
 * (`?ticket=…`), while the local listener does not register the minting route
 * at all. So the honest question is not "is a gateway tier enabled somewhere"
 * — it is "does *this* listener accept tickets", and the mint response answers
 * it directly: `201` means remote, `404`/`405` means local.
 *
 * The mode is latched on the first answer. Until then, concurrent streams each
 * attempt their own mint, which is what a remote session needs anyway (tickets
 * are single-use, never shared) and costs one wasted request per stream exactly
 * once on a local session.
 */
export type GatewayStreamAuthMode = "local" | "remote";

export class GatewayStreamAuthError extends Error {
  constructor(
    message: string,
    readonly status: number
  ) {
    super(message);
    this.name = "GatewayStreamAuthError";
  }
}

const TICKET_QUERY_PARAM = "ticket";

let mode: GatewayStreamAuthMode | undefined;

/** Resets the latched mode. Test-only seam. */
export function resetGatewayStreamAuth(): void {
  mode = undefined;
}

/** The latched mode, or `undefined` before the first mint answered. */
export function gatewayStreamAuthMode(): GatewayStreamAuthMode | undefined {
  return mode;
}

/**
 * Mints one ticket for one connection attempt, or returns `null` when this
 * listener is local and needs none. Never returns a previously issued ticket:
 * every connect and every reconnect gets its own.
 */
export async function acquireStreamTicket(signal?: AbortSignal): Promise<string | null> {
  if (mode === "local") return null;

  const { data, error, response } = await apiClient.POST("/api/gateway/stream-tickets", {
    signal,
  });

  if (response.status === 404 || response.status === 405) {
    mode = "local";
    return null;
  }

  mode = "remote";

  if (!response.ok || error !== undefined) {
    // An ended session is reported once, by the shared api-client middleware.
    throw new GatewayStreamAuthError(
      `Failed to authorize the live stream: ${response.status}`,
      response.status
    );
  }

  const rawTicket = data?.ticket;
  if (typeof rawTicket !== "string") {
    throw new GatewayStreamAuthError(
      `Failed to authorize the live stream: invalid ticket (${response.status})`,
      response.status
    );
  }
  const ticket = rawTicket.trim();
  if (!ticket) {
    throw new GatewayStreamAuthError(
      `Failed to authorize the live stream: empty ticket (${response.status})`,
      response.status
    );
  }
  return ticket;
}

/**
 * Returns `url` unchanged on a local same-origin session, or with a freshly
 * minted single-use ticket appended on a remote gateway session.
 */
export async function authorizeStreamUrl(url: string, signal?: AbortSignal): Promise<string> {
  const ticket = await acquireStreamTicket(signal);
  return ticket === null ? url : appendStreamTicket(url, ticket);
}

/**
 * Authorizes a streaming `fetch` target. The session prompt endpoint is a POST
 * that streams its response, so the daemon classifies it as a stream and
 * authenticates it with a ticket rather than the device credential.
 */
export async function authorizeStreamFetchInput(
  input: RequestInfo | URL,
  signal?: AbortSignal
): Promise<RequestInfo | URL> {
  const ticket = await acquireStreamTicket(signal);
  if (ticket === null) return input;
  if (typeof input === "string") return appendStreamTicket(input, ticket);
  if (input instanceof URL) return new URL(appendStreamTicket(input.toString(), ticket));
  return new Request(appendStreamTicket(input.url, ticket), input);
}

/** Appends the ticket to a relative or absolute URL without disturbing its other params. */
export function appendStreamTicket(url: string, ticket: string): string {
  const [beforeHash, hash] = splitHash(url);
  const separator = beforeHash.includes("?") ? "&" : "?";
  const authorized = `${beforeHash}${separator}${TICKET_QUERY_PARAM}=${encodeURIComponent(ticket)}`;
  return hash === undefined ? authorized : `${authorized}#${hash}`;
}

function splitHash(url: string): [string, string | undefined] {
  const index = url.indexOf("#");
  return index === -1 ? [url, undefined] : [url.slice(0, index), url.slice(index + 1)];
}
