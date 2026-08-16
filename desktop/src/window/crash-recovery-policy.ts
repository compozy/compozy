const CRASH_WINDOW_MS = 60_000;
const MAXIMUM_AUTOMATIC_RELOADS = 3;

export class CrashRecoveryBudget {
  readonly #timestamps: number[] = [];

  record(now: number): "reload" | "dialog" {
    while (this.#timestamps[0] !== undefined && now - this.#timestamps[0] >= CRASH_WINDOW_MS) {
      this.#timestamps.shift();
    }
    if (this.#timestamps.length >= MAXIMUM_AUTOMATIC_RELOADS) return "dialog";
    this.#timestamps.push(now);
    return "reload";
  }
}
