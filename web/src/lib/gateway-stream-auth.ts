import { apiClient } from "./api-client";
import { readGatewayListenerTier } from "./gateway-access-signal";
import { isHostedBrowserMode } from "./hosted-browser-auth";

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
const HOSTED_TICKET_QUERY_PARAM = "remote_ticket";

interface ListenerDiscovery {
  controller: AbortController;
  promise: Promise<GatewayStreamAuthMode>;
  waiters: number;
}

let mode: GatewayStreamAuthMode | undefined;
let discovery: ListenerDiscovery | undefined;

/** Resets the latched mode. Test-only seam. */
export function resetGatewayStreamAuth(): void {
  mode = undefined;
  const pending = discovery;
  discovery = undefined;
  pending?.controller.abort();
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
  signal?.throwIfAborted();
  if (mode) return mode;

  const pending = discovery ?? startListenerDiscovery();
  pending.waiters += 1;
  let onAbort: (() => void) | undefined;
  try {
    if (!signal) return await pending.promise;

    return await new Promise<GatewayStreamAuthMode>((resolve, reject) => {
      onAbort = () => reject(signal.reason);
      signal.addEventListener("abort", onAbort, { once: true });
      void pending.promise.then(resolve, reject);
    });
  } finally {
    if (onAbort) signal?.removeEventListener("abort", onAbort);
    pending.waiters -= 1;
    if (pending.waiters === 0 && discovery === pending) {
      discovery = undefined;
      pending.controller.abort();
    }
  }
}

function startListenerDiscovery(): ListenerDiscovery {
  // Each caller can stop waiting without canceling the other live connections.
  const controller = new AbortController();
  const pending: ListenerDiscovery = {
    controller,
    promise: readListenerMode(controller.signal),
    waiters: 0,
  };
  discovery = pending;
  void pending.promise.then(
    resolvedMode => {
      if (discovery !== pending) return;
      mode = resolvedMode;
      discovery = undefined;
    },
    () => {
      if (discovery === pending) discovery = undefined;
    }
  );
  return pending;
}

async function readListenerMode(signal: AbortSignal): Promise<GatewayStreamAuthMode> {
  const { response } = await apiClient.GET("/api/status", { signal });
  const tier = readGatewayListenerTier(response);
  if (!response.ok || !tier) {
    throw new GatewayStreamAuthError(
      `Failed to identify the live stream listener: ${response.status}`,
      response.status
    );
  }

  return tier === "local" ? "local" : "remote";
}

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
    body: { path },
    signal,
  });

  if (!response.ok || error !== undefined) {
    throw new GatewayStreamAuthError(
      `Failed to authorize the hosted live stream: ${response.status}`,
      response.status
    );
  }

  const rawTicket = data?.ticket;
  if (typeof rawTicket !== "string" || rawTicket.trim() === "") {
    throw new GatewayStreamAuthError(
      `Failed to authorize the hosted live stream: invalid ticket (${response.status})`,
      response.status
    );
  }

  return rawTicket.trim();
}

/**
 * Returns `url` unchanged on a local same-origin session, with `?remote_ticket=`
 * on a hosted browser session, or with `?ticket=` on a legacy gateway session.
 */
export async function authorizeStreamUrl(url: string, signal?: AbortSignal): Promise<string> {
  if (isHostedBrowserMode()) {
    const ticket = await acquireHostedStreamTicket(url, signal);
    return appendHostedStreamTicket(url, ticket);
  }
  const ticket = await acquireStreamTicket(signal);
  return ticket === null ? url : appendStreamTicket(url, ticket);
}

/**
 * Authorizes a streaming `fetch` target.
 *
 * On hosted browser sessions, POST streams (such as session prompt) authenticate
 * directly via the browser session cookie + DPoP header attached by `daemonFetch`,
 * while `POST /remote/v1/stream-tickets` only authorizes GET SSE/WebSocket streams.
 */
export async function authorizeStreamFetchInput(
  input: RequestInfo | URL,
  signal?: AbortSignal
): Promise<RequestInfo | URL> {
  if (isHostedBrowserMode()) return input;
  const ticket = await acquireStreamTicket(signal);
  if (ticket === null) return input;
  if (typeof input === "string") return appendStreamTicket(input, ticket);
  if (input instanceof URL) return new URL(appendStreamTicket(input.toString(), ticket));
  return new Request(appendStreamTicket(input.url, ticket), input);
}

/** Appends a gateway `?ticket=` to a relative or absolute URL without disturbing other params. */
export function appendStreamTicket(url: string, ticket: string): string {
  return appendNamedStreamTicket(url, TICKET_QUERY_PARAM, ticket);
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
