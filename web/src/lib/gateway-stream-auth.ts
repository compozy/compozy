import { apiClient } from "./api-client";
import { readGatewayListenerTier } from "./gateway-access-signal";

/**
 * Stream authentication for the page's own origin.
 *
 * A tier listener authenticates streams with a single-use ticket
 * (`?ticket=…`), while the local listener needs none. Every API response
 * identifies the physical listener through `X-Compozy-Gateway-Tier`, so the
 * client never probes an intentionally absent route to infer its mode.
 *
 * The mode is latched after an explicit status read. Remote connections still
 * mint one ticket per connect and reconnect; tickets are single-use and never
 * shared.
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

/** The latched mode, or `undefined` before listener discovery completes. */
export function gatewayStreamAuthMode(): GatewayStreamAuthMode | undefined {
  return mode;
}

/**
 * Mints one ticket for one connection attempt, or returns `null` when this
 * listener is local and needs none. Never returns a previously issued ticket:
 * every connect and every reconnect gets its own.
 */
export async function acquireStreamTicket(signal?: AbortSignal): Promise<string | null> {
  const resolvedMode = await resolveGatewayStreamAuthMode(signal);
  if (resolvedMode === "local") return null;

  const { data, error, response } = await apiClient.POST("/api/gateway/stream-tickets", {
    signal,
  });

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

async function resolveGatewayStreamAuthMode(signal?: AbortSignal): Promise<GatewayStreamAuthMode> {
  if (mode) return mode;

  const { response } = await apiClient.GET("/api/status", { signal });
  const tier = readGatewayListenerTier(response);
  if (!response.ok || !tier) {
    throw new GatewayStreamAuthError(
      `Failed to identify the live stream listener: ${response.status}`,
      response.status
    );
  }

  mode = tier === "local" ? "local" : "remote";
  return mode;
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
