import { mkdir, realpath } from "node:fs/promises";

import type { ConnectOptions } from "./connect.js";
import { CompozyConfigError, CompozyUnavailableError } from "./errors.js";
import { Compozy as Client } from "./index-node.js";
import { resolveBinary } from "./node-binary.js";
import { daemonBaseUrl, readDaemonInfo, resolveHome } from "./node-discovery.js";
import {
  launchDaemon,
  launchError,
  stopDaemon,
  terminateLaunch,
  waitForLaunchExit,
  type DaemonLaunch,
} from "./node-process.js";

export * from "./index-node.js";

export interface StartOptions extends Omit<ConnectOptions, "baseUrl"> {
  binaryPath?: string;
}

export interface CloseOptions {
  stopDaemon?: boolean;
}

interface OwnedDaemon {
  launch: DaemonLaunch;
  home: string;
  pid: number;
  timeoutMs: number;
}

/** Node-only client that can start a daemon and stop the daemon it started. */
export class Compozy extends Client {
  private ownedDaemon: OwnedDaemon | undefined;
  private closing: Promise<void> | undefined;

  static override async connect(options: ConnectOptions = {}): Promise<Compozy> {
    return new Compozy(await this.connection(options));
  }

  static async start(options: StartOptions = {}): Promise<Compozy> {
    const timeoutMs = options.timeoutMs ?? 30_000;
    if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
      throw new CompozyConfigError("start timeoutMs must be a positive number.");
    }
    if (options.home !== undefined && !options.home.trim()) {
      throw new CompozyConfigError("start home must be a non-empty directory path.");
    }
    const deadline = Date.now() + timeoutMs;
    const home = await prepareHome(resolveHome(options.home));
    const existing = await this.connectHome(home, options, deadline);
    if (existing) return existing;

    const binary = await resolveBinary(options.binaryPath);
    if (Date.now() >= deadline) {
      throw new CompozyUnavailableError(`compozy daemon start timed out after ${timeoutMs}ms.`);
    }
    // Foreground mode avoids a second, unowned fork. The SDK detaches this exact
    // process after readiness, and can terminate it even before daemon.json exists.
    const launch = await launchDaemon(binary, home);
    try {
      while (Date.now() < deadline) {
        if (launch.error || launch.exited) {
          throw await launchError(launch, "exited before readiness");
        }
        const client = await this.connectHome(home, options, deadline, launch.child.pid);
        if (client && !launch.exited && !launch.error) {
          client.ownedDaemon = { home, pid: client.daemon.pid, timeoutMs, launch };
          launch.child.unref();
          return client;
        }
        await new Promise(resolveDelay =>
          setTimeout(resolveDelay, Math.min(50, Math.max(0, deadline - Date.now())))
        );
      }
      throw await launchError(launch, `timed out after ${timeoutMs}ms waiting for readiness`);
    } catch (cause) {
      await terminateLaunch(launch);
      throw cause;
    }
  }

  /** stopDaemon is effective only while this home's record still names our daemon. */
  async close(options: CloseOptions = {}): Promise<void> {
    if (!options.stopDaemon || !this.ownedDaemon) return;
    if (this.closing) return this.closing;
    this.closing = this.stopOwnedDaemon();
    try {
      await this.closing;
    } finally {
      this.closing = undefined;
    }
  }

  private async stopOwnedDaemon(): Promise<void> {
    const owned = this.ownedDaemon;
    if (!owned) return;
    const info = await readDaemonInfo(owned.home);
    if (info && info.pid !== owned.pid) {
      this.ownedDaemon = undefined;
      return;
    }
    const deadline = Date.now() + owned.timeoutMs;
    if (info) await stopDaemon(owned.launch, owned.timeoutMs);
    await waitForLaunchExit(owned.launch, Math.max(0, deadline - Date.now()));
    this.ownedDaemon = undefined;
  }

  private static async connectHome(
    home: string,
    options: StartOptions,
    deadline: number,
    expectedPid?: number
  ): Promise<Compozy | undefined> {
    const info = await readDaemonInfo(home);
    if (!info || (expectedPid !== undefined && info.pid !== expectedPid)) return undefined;
    try {
      const client = new Compozy(
        await this.connection({
          ...options,
          home,
          baseUrl: daemonBaseUrl(info),
          timeoutMs: Math.max(1, Math.min(500, deadline - Date.now())),
        })
      );
      if (client.daemon.pid !== info.pid) {
        throw new CompozyConfigError(
          "Daemon identity does not match the requested home's daemon.json pid."
        );
      }
      // Readiness and ownership must describe the same record, even during a restart.
      if ((await readDaemonInfo(home))?.pid !== info.pid) return undefined;
      return client;
    } catch (error) {
      if (error instanceof CompozyUnavailableError) return undefined;
      throw error;
    }
  }
}

async function prepareHome(home: string): Promise<string> {
  try {
    await mkdir(home, { recursive: true, mode: 0o700 });
    return await realpath(home);
  } catch (cause) {
    throw new CompozyConfigError(
      `Cannot use daemon home ${home}: expected an accessible directory.`,
      { cause }
    );
  }
}
