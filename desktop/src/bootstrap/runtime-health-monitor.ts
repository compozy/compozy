const DEFAULT_INTERVAL_MS = 2_000;
const DEFAULT_TIMEOUT_MS = 1_500;
const DEFAULT_FAILURE_THRESHOLD = 3;
// A settings restart or runtime update stops the daemon, waits for its resources to release, and
// starts a replacement that may migrate data first. The shell waits this long before treating the
// outage as a lost runtime that needs bootstrap.
const DEFAULT_RECONNECT_TIMEOUT_MS = 45_000;

type Fetcher = (input: string, init: RequestInit) => Promise<Response>;

/**
 * Watches the runtime the product window is bound to. An outage first reports `onDisconnected`
 * while probing continues; the runtime answering again reports `onReconnected` until it succeeds, and an outage that
 * outlasts the reconnect window stops the monitor and reports `onLost`.
 */
export class RuntimeHealthMonitor {
  readonly #identityURL: string;
  readonly #fetcher: Fetcher;
  readonly #intervalMs: number;
  readonly #timeoutMs: number;
  readonly #failureThreshold: number;
  readonly #reconnectTimeoutMs: number;
  readonly #now: () => number;
  readonly #onDisconnected: () => Promise<void>;
  readonly #onReconnected: () => Promise<void>;
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
    now?: () => number;
    onDisconnected: () => Promise<void>;
    onReconnected: () => Promise<void>;
    onLost: () => Promise<void>;
    onError?: (error: unknown) => void;
  }) {
    this.#identityURL = new URL("/api/status/identity", options.origin).toString();
    this.#fetcher = options.fetcher ?? fetch;
    this.#intervalMs = options.intervalMs ?? DEFAULT_INTERVAL_MS;
    this.#timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    this.#failureThreshold = options.failureThreshold ?? DEFAULT_FAILURE_THRESHOLD;
    this.#reconnectTimeoutMs = options.reconnectTimeoutMs ?? DEFAULT_RECONNECT_TIMEOUT_MS;
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
      const reachable = await this.#reachable();
      if (!this.#timer) return;
      if (reachable) await this.#recordSuccess();
      else await this.#recordFailure();
    } finally {
      this.#inFlight = false;
    }
  }

  async #reachable(): Promise<boolean> {
    try {
      const response = await this.#fetcher(this.#identityURL, {
        method: "GET",
        signal: AbortSignal.timeout(this.#timeoutMs),
      });
      return response.ok;
    } catch {
      return false;
    }
  }

  async #recordSuccess(): Promise<void> {
    this.#failures = 0;
    if (this.#disconnectedAt === null) return;
    // A failed reconnection (such as a reload that raced another drop) stays pending, so the next
    // healthy probe retries it instead of leaving the window on a failed page.
    await this.#onReconnected();
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
    if (this.#now() - this.#disconnectedAt < this.#reconnectTimeoutMs) return;
    this.stop();
    await this.#onLost();
  }
}
