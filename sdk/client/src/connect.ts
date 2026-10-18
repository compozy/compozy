import {
  CompozyApiError,
  CompozyConfigError,
  CompozyUnavailableError,
  responseError,
} from "./errors.js";
import { CLIENT_VERSION } from "./version.js";

export interface ConnectOptions {
  baseUrl?: string;
  token?: string;
  home?: string;
  timeoutMs?: number;
  fetch?: typeof globalThis.fetch;
  clientId?: string;
  onVersionMismatch?: (versions: { client: string; daemon: string }) => void;
}

export interface DaemonIdentity {
  version: string;
  httpHost: string;
  httpPort: number;
  pid: number;
  userHomeDir: string;
  startedAt: string;
}

export interface ConnectedState {
  baseUrl: string;
  headers: Headers;
  fetch: typeof globalThis.fetch;
  daemon: DaemonIdentity;
}

export type Discovery = (options: ConnectOptions) => Promise<{ baseUrl: string; token?: string }>;

export async function connect(
  options: ConnectOptions,
  discover: Discovery
): Promise<ConnectedState> {
  const target = await discover(options);
  const timeoutMs = options.timeoutMs ?? 5000;
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
    throw new CompozyConfigError("connect timeoutMs must be a positive number.");
  }
  const headers = new Headers({ "X-Compozy-Client-ID": options.clientId ?? "sdk-ts" });
  const token = options.token ?? target.token;
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const transport = options.fetch ?? globalThis.fetch;
  if (!transport)
    throw new CompozyConfigError("This runtime must provide fetch or a fetch option.");
  const fetch: typeof globalThis.fetch = async (input, init) => {
    try {
      return await transport(input, init);
    } catch (cause) {
      if (init?.signal?.aborted || (input instanceof Request && input.signal.aborted)) throw cause;
      throw new CompozyUnavailableError(
        "CompozyOS daemon is unavailable. Run `compozy daemon start`.",
        { cause }
      );
    }
  };
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(`${target.baseUrl}/api/status/identity`, {
      headers,
      signal: controller.signal,
    });
    if (!response.ok) throw await responseError(response);
    const identity = decodeIdentity(await response.json());
    checkVersion(identity.version, options.onVersionMismatch);
    return { baseUrl: target.baseUrl, headers, fetch, daemon: identity };
  } catch (cause) {
    if (cause instanceof CompozyApiError || cause instanceof CompozyConfigError) throw cause;
    throw new CompozyUnavailableError(
      "CompozyOS daemon is unavailable. Run `compozy daemon start`.",
      { cause }
    );
  } finally {
    clearTimeout(timeout);
  }
}

function decodeIdentity(body: unknown): DaemonIdentity {
  if (
    !body ||
    typeof body !== "object" ||
    !("daemon" in body) ||
    !body.daemon ||
    typeof body.daemon !== "object"
  ) {
    throw new CompozyConfigError("Daemon identity response is invalid.");
  }
  const daemon = body.daemon as Record<string, unknown>;
  if (
    typeof daemon.version !== "string" ||
    typeof daemon.http_host !== "string" ||
    typeof daemon.http_port !== "number" ||
    typeof daemon.pid !== "number"
  ) {
    throw new CompozyConfigError(
      "Daemon identity response is missing its version or listener identity."
    );
  }
  return {
    version: daemon.version,
    httpHost: daemon.http_host,
    httpPort: daemon.http_port,
    pid: daemon.pid,
    userHomeDir: typeof daemon.user_home_dir === "string" ? daemon.user_home_dir : "",
    startedAt: typeof daemon.started_at === "string" ? daemon.started_at : "",
  };
}

export function checkVersion(
  daemon: string,
  onMismatch: ConnectOptions["onVersionMismatch"],
  client = CLIENT_VERSION
): void {
  if (!client || !daemon || client === "0.0.0-dev" || daemon === "0.0.0-dev") return;
  const majorMinor = (version: string) =>
    /^v?(\d+)\.(\d+)(?:\.|$)/.exec(version)?.slice(1).join(".");
  const clientVersion = majorMinor(client);
  const daemonVersion = majorMinor(daemon);
  if (!clientVersion || !daemonVersion || clientVersion === daemonVersion) return;
  (
    onMismatch ??
    (versions =>
      console.warn(`CompozyOS client ${versions.client} differs from daemon ${versions.daemon}.`))
  )({ client, daemon });
}
