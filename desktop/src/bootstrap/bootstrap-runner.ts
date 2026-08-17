import { spawn } from "node:child_process";
import { appendFile } from "node:fs/promises";
import { dirname } from "node:path";
import { createInterface } from "node:readline";

import {
  parseBootstrapEvent,
  type BootstrapEvent,
  type BootstrapResolution,
} from "./bootstrap-event";
import { verifyRuntimeBundle } from "./bundle-integrity";
import { ensurePrivateDirectory } from "../files/private-directory";

const MAXIMUM_STDERR_BYTES = 64 * 1024;

export type BootstrapResult = NonNullable<BootstrapEvent["daemon"]> & {
  readonly resolution: BootstrapResolution;
};

export class BootstrapRunner {
  readonly #bundlePath: string;
  readonly #manifestPath: string;
  readonly #logPath: string;
  readonly #minimumRuntime: string;
  readonly #appVersion: string;
  readonly #channel: "beta" | "development";
  readonly #bootId: string;
  readonly #environment: NodeJS.ProcessEnv;
  #running: Promise<BootstrapResult> | null = null;

  constructor(options: {
    bundlePath: string;
    manifestPath: string;
    logPath: string;
    minimumRuntime: string;
    appVersion: string;
    channel: "beta" | "development";
    bootId: string;
    environment?: NodeJS.ProcessEnv;
  }) {
    this.#bundlePath = options.bundlePath;
    this.#manifestPath = options.manifestPath;
    this.#logPath = options.logPath;
    this.#minimumRuntime = options.minimumRuntime;
    this.#appVersion = options.appVersion;
    this.#channel = options.channel;
    this.#bootId = options.bootId;
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
    await ensurePrivateDirectory(dirname(this.#logPath));
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
        "--channel",
        this.#channel,
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
      const event = parseBootstrapEvent(line);
      if (!event) throw new Error("The runtime bootstrap returned an invalid event.");
      await appendFile(
        this.#logPath,
        `${JSON.stringify({
          timestamp: new Date().toISOString(),
          level: event.status === "failed" ? "ERROR" : "INFO",
          boot_id: this.#bootId,
          message: event.message,
          phase: event.phase,
          status: event.status,
          ...(event.resolution ? { resolution: event.resolution } : {}),
          ...(event.daemon ? { daemon: event.daemon } : {}),
        })}\n`,
        { encoding: "utf8", mode: 0o600 }
      );
      await onEvent(event);
      if (event.phase === "ready" && event.status === "completed" && event.daemon) {
        if (!event.resolution) throw new Error("The runtime bootstrap omitted its resolution.");
        ready = { ...event.daemon, resolution: event.resolution };
      }
    }
    const exitCode = await exit;
    if (exitCode !== 0 || !ready) {
      throw new Error(stderr.trim() || "The CompozyOS runtime could not start.");
    }
    return ready;
  }
}
