import { spawn } from "node:child_process";
import { appendFile, chmod, mkdir } from "node:fs/promises";
import { dirname } from "node:path";
import { createInterface } from "node:readline";

import { parseBootstrapEvent, type BootstrapEvent } from "./bootstrap-event";
import { verifyRuntimeBundle } from "./bundle-integrity";

const MAXIMUM_STDERR_BYTES = 64 * 1024;

export interface BootstrapResult {
  readonly origin: string;
  readonly version: string;
  readonly pid: number;
}

export class BootstrapRunner {
  readonly #bundlePath: string;
  readonly #manifestPath: string;
  readonly #logPath: string;
  readonly #minimumRuntime: string;
  readonly #appVersion: string;
  readonly #environment: NodeJS.ProcessEnv;
  #running: Promise<BootstrapResult> | null = null;

  constructor(options: {
    bundlePath: string;
    manifestPath: string;
    logPath: string;
    minimumRuntime: string;
    appVersion: string;
    environment?: NodeJS.ProcessEnv;
  }) {
    this.#bundlePath = options.bundlePath;
    this.#manifestPath = options.manifestPath;
    this.#logPath = options.logPath;
    this.#minimumRuntime = options.minimumRuntime;
    this.#appVersion = options.appVersion;
    this.#environment = options.environment ?? process.env;
  }

  async run(onEvent: (event: BootstrapEvent) => Promise<void>): Promise<BootstrapResult> {
    if (this.#running) return await this.#running;
    const attempt = this.#execute(onEvent);
    this.#running = attempt;
    try {
      return await attempt;
    } finally {
      this.#running = null;
    }
  }

  async #execute(onEvent: (event: BootstrapEvent) => Promise<void>): Promise<BootstrapResult> {
    await verifyRuntimeBundle(this.#bundlePath, this.#manifestPath);
    await mkdir(dirname(this.#logPath), { recursive: true, mode: 0o700 });
    await chmod(dirname(this.#logPath), 0o700);
    const child = spawn(
      this.#bundlePath,
      [
        "daemon",
        "bootstrap",
        "--bundle-path",
        this.#bundlePath,
        "--minimum-runtime",
        this.#minimumRuntime,
        "--app-version",
        this.#appVersion,
        "-o",
        "jsonl",
      ],
      {
        env: this.#environment,
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
      }
    );
    if (!child.stdout || !child.stderr)
      throw new Error("The bootstrap process streams are unavailable.");
    const exit = new Promise<number | null>((resolve, reject) => {
      child.once("error", reject);
      child.once("close", resolve);
    });
    let ready: BootstrapResult | null = null;
    let stderr = "";
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk: string) => {
      stderr = `${stderr}${chunk}`.slice(-MAXIMUM_STDERR_BYTES);
    });
    const output = createInterface({ input: child.stdout, crlfDelay: Infinity });
    for await (const line of output) {
      await appendFile(this.#logPath, `${line}\n`, { encoding: "utf8", mode: 0o600 });
      const event = parseBootstrapEvent(line);
      if (!event) throw new Error("The runtime bootstrap returned an invalid event.");
      await onEvent(event);
      if (event.phase === "ready" && event.status === "completed" && event.daemon) {
        ready = event.daemon;
      }
    }
    const exitCode = await exit;
    if (exitCode !== 0 || !ready) {
      throw new Error(stderr.trim() || "The CompozyOS runtime could not start.");
    }
    return ready;
  }
}
