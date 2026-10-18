import { spawn, type ChildProcess } from "node:child_process";
import { mkdir, open } from "node:fs/promises";
import { join } from "node:path";

import { CompozyUnavailableError } from "./errors.js";

interface DaemonLaunch {
  child: ChildProcess;
  logPath: string;
  logOffset: number;
  error?: Error;
  exited?: { code: number | null; signal: NodeJS.Signals | null };
}

function environment(home: string): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { ...process.env, COMPOZY_HOME: home };
  // Lifecycle commands are always scoped to their home, never a remote URL.
  delete env.COMPOZY_URL;
  return env;
}

export async function launchDaemon(binary: string, home: string): Promise<DaemonLaunch> {
  const logPath = join(home, "logs", "daemon-stdio.log");
  await mkdir(join(home, "logs"), { recursive: true, mode: 0o700 });
  const log = await open(logPath, "a", 0o600);
  try {
    const logOffset = (await log.stat()).size;
    const child = spawn(binary, ["daemon", "start", "--foreground"], {
      env: environment(home),
      stdio: ["ignore", log.fd, log.fd],
      detached: true,
      windowsHide: true,
    });
    const launch: DaemonLaunch = { child, logPath, logOffset };
    child.once("error", error => {
      launch.error = error;
    });
    child.once("exit", (code, signal) => {
      launch.exited = { code, signal };
    });
    return launch;
  } finally {
    await log.close();
  }
}

export async function launchError(
  launch: DaemonLaunch,
  reason: string
): Promise<CompozyUnavailableError> {
  let tail = "";
  try {
    const log = await open(launch.logPath, "r");
    try {
      const size = (await log.stat()).size;
      const start = Math.max(launch.logOffset, size - 8192);
      const bytes = Buffer.alloc(Math.max(0, size - start));
      const { bytesRead } = await log.read(bytes, 0, bytes.length, start);
      tail = bytes.subarray(0, bytesRead).toString("utf8").trim();
    } finally {
      await log.close();
    }
  } catch {
    // Preserve the startup error even when the daemon removed or rotated its log.
  }
  const failure = launch.error?.message;
  return new CompozyUnavailableError(
    `compozy daemon start ${reason}.${failure ? ` ${failure}` : ""}${tail ? `\n${tail}` : ""}`,
    launch.error ? { cause: launch.error } : undefined
  );
}

export async function terminateLaunch(launch: DaemonLaunch): Promise<void> {
  const { child } = launch;
  if (!child.pid || launch.exited) return;
  await new Promise<void>((resolveExit, reject) => {
    const timer = setTimeout(() => {
      child.kill("SIGKILL");
    }, 500);
    child.once("exit", () => {
      clearTimeout(timer);
      resolveExit();
    });
    try {
      child.kill("SIGTERM");
    } catch (cause) {
      clearTimeout(timer);
      reject(cause);
    }
  });
}

export function stopDaemon(binary: string, home: string, timeoutMs: number): Promise<void> {
  return new Promise((resolveStop, reject) => {
    let stderr = "";
    let timedOut = false;
    const child = spawn(binary, ["daemon", "stop"], {
      env: environment(home),
      stdio: ["ignore", "ignore", "pipe"],
      windowsHide: true,
    });
    child.stderr?.setEncoding("utf8");
    child.stderr?.on("data", (chunk: string) => {
      stderr = (stderr + chunk).slice(-8192);
    });
    const timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGKILL");
    }, timeoutMs);
    child.once("error", cause => {
      clearTimeout(timer);
      reject(
        new CompozyUnavailableError(`Unable to run compozy daemon stop: ${cause.message}`, {
          cause,
        })
      );
    });
    child.once("close", (code, signal) => {
      clearTimeout(timer);
      if (code === 0 && !timedOut) resolveStop();
      else {
        const reason = timedOut
          ? `timed out after ${timeoutMs}ms`
          : `exited with ${signal ?? code}`;
        reject(
          new CompozyUnavailableError(
            `compozy daemon stop ${reason}.${stderr.trim() ? `\n${stderr.trim()}` : ""}`
          )
        );
      }
    });
  });
}
