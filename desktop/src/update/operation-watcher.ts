import { watch, type FSWatcher } from "node:fs";
import { readFile } from "node:fs/promises";
import { basename, dirname } from "node:path";

import { parseUpdateOperation, type UpdateOperation } from "./operation-contract";

const MAXIMUM_OPERATION_BYTES = 1024 * 1024;

function isMissingFile(error: unknown): boolean {
  return error instanceof Error && "code" in error && error.code === "ENOENT";
}

export class OperationWatcher {
  readonly #path: string;
  readonly #pollIntervalMs: number;
  readonly #onOperation: (operation: UpdateOperation | null) => Promise<void>;
  readonly #onError: (error: Error) => void;
  #watcher: FSWatcher | null = null;
  #timer: NodeJS.Timeout | null = null;
  #reading = false;
  #rerun = false;
  #lastRevision = "";

  constructor(options: {
    path: string;
    pollIntervalMs: number;
    onOperation: (operation: UpdateOperation | null) => Promise<void>;
    onError: (error: Error) => void;
  }) {
    this.#path = options.path;
    this.#pollIntervalMs = options.pollIntervalMs;
    this.#onOperation = options.onOperation;
    this.#onError = options.onError;
  }

  start(): void {
    if (this.#timer) return;
    const parent = dirname(this.#path);
    const filename = basename(this.#path);
    try {
      this.#watcher = watch(parent, (_event, changed) => {
        if (changed === null || changed === filename) void this.#readLatest();
      });
      this.#watcher.on("error", error => this.#onError(error));
    } catch (error) {
      this.#onError(error instanceof Error ? error : new Error(String(error)));
    }
    this.#timer = setInterval(() => void this.#readLatest(), this.#pollIntervalMs);
    this.#timer.unref();
    void this.#readLatest();
  }

  stop(): void {
    this.#watcher?.close();
    this.#watcher = null;
    if (this.#timer) clearInterval(this.#timer);
    this.#timer = null;
  }

  async #readLatest(): Promise<void> {
    if (this.#reading) {
      this.#rerun = true;
      return;
    }
    this.#reading = true;
    try {
      do {
        this.#rerun = false;
        await this.#readOnce();
      } while (this.#rerun);
    } finally {
      this.#reading = false;
    }
  }

  async #readOnce(): Promise<void> {
    try {
      const data = await readFile(this.#path);
      if (data.byteLength > MAXIMUM_OPERATION_BYTES) {
        throw new Error("The update operation exceeds its size limit.");
      }
      const operation = parseUpdateOperation(JSON.parse(data.toString("utf8")) as unknown);
      const revision = `${operation.operation_id}:${operation.revision}`;
      if (revision === this.#lastRevision) return;
      this.#lastRevision = revision;
      await this.#onOperation(operation);
    } catch (error) {
      if (isMissingFile(error)) {
        if (this.#lastRevision !== "missing") {
          this.#lastRevision = "missing";
          await this.#onOperation(null);
        }
        return;
      }
      this.#onError(error instanceof Error ? error : new Error(String(error)));
    }
  }
}
