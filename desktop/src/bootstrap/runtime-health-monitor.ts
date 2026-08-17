const DEFAULT_INTERVAL_MS = 2_000;
const DEFAULT_TIMEOUT_MS = 1_500;
const DEFAULT_FAILURE_THRESHOLD = 3;

type Fetcher = (input: string, init: RequestInit) => Promise<Response>;

export class RuntimeHealthMonitor {
  readonly #identityURL: string;
  readonly #fetcher: Fetcher;
  readonly #intervalMs: number;
  readonly #timeoutMs: number;
  readonly #failureThreshold: number;
  readonly #onDisconnected: () => Promise<void>;
  #timer: NodeJS.Timeout | null = null;
  #inFlight = false;
  #failures = 0;

  constructor(options: {
    origin: string;
    fetcher?: Fetcher;
    intervalMs?: number;
    timeoutMs?: number;
    failureThreshold?: number;
    onDisconnected: () => Promise<void>;
  }) {
    this.#identityURL = new URL("/api/status/identity", options.origin).toString();
    this.#fetcher = options.fetcher ?? fetch;
    this.#intervalMs = options.intervalMs ?? DEFAULT_INTERVAL_MS;
    this.#timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    this.#failureThreshold = options.failureThreshold ?? DEFAULT_FAILURE_THRESHOLD;
    this.#onDisconnected = options.onDisconnected;
  }

  start(): void {
    if (this.#timer) return;
    this.#timer = setInterval(() => void this.#probe(), this.#intervalMs);
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
      const response = await this.#fetcher(this.#identityURL, {
        method: "GET",
        signal: AbortSignal.timeout(this.#timeoutMs),
      });
      if (!response.ok) throw new Error(`Runtime identity returned HTTP ${response.status}.`);
      this.#failures = 0;
    } catch {
      this.#failures += 1;
      if (this.#failures >= this.#failureThreshold) {
        this.stop();
        await this.#onDisconnected();
      }
    } finally {
      this.#inFlight = false;
    }
  }
}
