const DEFAULT_INTERVAL_MS = 2_000;
const DEFAULT_TIMEOUT_MS = 1_500;
const DEFAULT_FAILURE_THRESHOLD = 3;
// A settings restart or runtime update stops the daemon, waits for its resources to release, and
// starts a replacement that may migrate data first. The shell waits this long before it asks
// whether a runtime process is still alive; a live one is never treated as lost on this clock.
const DEFAULT_RECONNECT_TIMEOUT_MS = 45_000;
// Mirrors the bootstrap's own bound on waiting for a booting runtime (bootstrapAttachReadyTimeout):
// past it a live process that never answers is handed to bootstrap, which reports what it finds.
const DEFAULT_MAXIMUM_WAIT_MS = 10 * 60_000;

type Fetcher = (input: string, init: RequestInit) => Promise<Response>;

/** The runtime process that answered a probe; null when the answer carried no usable identity. */
export interface RuntimeIdentity {
  readonly pid: number;
  readonly version: string;
}

async function readIdentity(response: Response): Promise<RuntimeIdentity | null> {
  try {
    const body: unknown = await response.json();
    const daemon = (body as { daemon?: { pid?: unknown; version?: unknown } } | null)?.daemon;
    if (typeof daemon?.pid !== "number" || typeof daemon.version !== "string") return null;
    return { pid: daemon.pid, version: daemon.version };
  } catch {
    return null;
  }
}

/**
 * Watches the runtime the product window is bound to. An outage first reports `onDisconnected`
 * while probing continues; the runtime answering again reports `onReconnected` until it succeeds.
 * An outage that outlasts the reconnect window reports `onLost` and stops the monitor only once
 * no runtime process is alive (`runtimeAlive`), or the maximum wait has passed.
 */
export class RuntimeHealthMonitor {
  readonly #identityURL: string;
  readonly #fetcher: Fetcher;
  readonly #intervalMs: number;
  readonly #timeoutMs: number;
  readonly #failureThreshold: number;
  readonly #reconnectTimeoutMs: number;
  readonly #maximumWaitMs: number;
  readonly #runtimeAlive: () => Promise<boolean>;
  readonly #now: () => number;
  readonly #onDisconnected: () => Promise<void>;
  readonly #onReconnected: (identity: RuntimeIdentity | null) => Promise<void>;
  readonly #onLost: () => Promise<void>;
  readonly #onError: (error: unknown) => void;
  #timer: NodeJS.Timeout | null = null;
  #inFlight = false;
  #failures = 0;
  #disconnectedAt: number | null = null;

  constructor(options: {
    origin: string;
    fetcher?: Fetcher;
    intervalMs?: number;
    timeoutMs?: number;
    failureThreshold?: number;
    reconnectTimeoutMs?: number;
    maximumWaitMs?: number;
    /** Whether a runtime process is still booting or restarting; defaults to "no". */
    runtimeAlive?: () => Promise<boolean>;
    now?: () => number;
    onDisconnected: () => Promise<void>;
    onReconnected: (identity: RuntimeIdentity | null) => Promise<void>;
    onLost: () => Promise<void>;
    onError?: (error: unknown) => void;
  }) {
    this.#identityURL = new URL("/api/status/identity", options.origin).toString();
    this.#fetcher = options.fetcher ?? fetch;
    this.#intervalMs = options.intervalMs ?? DEFAULT_INTERVAL_MS;
    this.#timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    this.#failureThreshold = options.failureThreshold ?? DEFAULT_FAILURE_THRESHOLD;
    this.#reconnectTimeoutMs = options.reconnectTimeoutMs ?? DEFAULT_RECONNECT_TIMEOUT_MS;
    this.#maximumWaitMs = options.maximumWaitMs ?? DEFAULT_MAXIMUM_WAIT_MS;
    this.#runtimeAlive = options.runtimeAlive ?? (() => Promise.resolve(false));
    this.#now = options.now ?? Date.now;
    this.#onDisconnected = options.onDisconnected;
    this.#onReconnected = options.onReconnected;
    this.#onLost = options.onLost;
    this.#onError = options.onError ?? (() => undefined);
  }

  start(): void {
    if (this.#timer) return;
    this.#timer = setInterval(
      () => void this.#probe().catch(error => this.#onError(error)),
      this.#intervalMs
    );
    this.#timer.unref();
  }

  stop(): void {
    if (this.#timer) clearInterval(this.#timer);
    this.#timer = null;
  }

  async #probe(): Promise<void> {
    if (this.#inFlight) return;
    this.#inFlight = true;
    try {
      const answer = await this.#reachable();
      if (!this.#timer) return;
      if (answer) await this.#recordSuccess(answer.identity);
      else await this.#recordFailure();
    } finally {
      this.#inFlight = false;
    }
  }

  async #reachable(): Promise<{ identity: RuntimeIdentity | null } | null> {
    try {
      const response = await this.#fetcher(this.#identityURL, {
        method: "GET",
        signal: AbortSignal.timeout(this.#timeoutMs),
      });
      if (!response.ok) return null;
      // The identity only matters to a reconnection; a healthy probe leaves the body unread.
      return { identity: this.#disconnectedAt === null ? null : await readIdentity(response) };
    } catch {
      return null;
    }
  }

  async #recordSuccess(identity: RuntimeIdentity | null): Promise<void> {
    this.#failures = 0;
    if (this.#disconnectedAt === null) return;
    // A failed reconnection (such as a reload that raced another drop) stays pending, so the next
    // healthy probe retries it instead of leaving the window on a failed page.
    await this.#onReconnected(identity);
    this.#disconnectedAt = null;
  }

  async #recordFailure(): Promise<void> {
    this.#failures += 1;
    if (this.#disconnectedAt === null) {
      if (this.#failures < this.#failureThreshold) return;
      this.#disconnectedAt = this.#now();
      await this.#onDisconnected();
      return;
    }
    const outage = this.#now() - this.#disconnectedAt;
    if (outage < this.#reconnectTimeoutMs) return;
    // A live runtime process is booting or restarting. Abandoning it would start a competing
    // bootstrap against a runtime that is about to answer.
    if (outage < this.#maximumWaitMs && (await this.#runtimeAlive())) return;
    if (!this.#timer) return;
    this.stop();
    await this.#onLost();
  }
}
