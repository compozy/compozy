import { readFile } from "node:fs/promises";
import { join } from "node:path";

/** True when a process with this pid exists; one this user may not signal still exists. */
export function processAlive(
  pid: number,
  signal: (pid: number, signal: 0) => unknown = process.kill.bind(process)
): boolean {
  if (!Number.isSafeInteger(pid) || pid < 1) return false;
  try {
    signal(pid, 0);
    return true;
  } catch (error) {
    return (error as NodeJS.ErrnoException).code === "EPERM";
  }
}

async function readText(path: string): Promise<string> {
  try {
    return (await readFile(path, "utf8")).trim();
  } catch {
    return "";
  }
}

/**
 * The pids the runtime records in its home while it runs or changes: the daemon lock holder (a
 * daemon records itself there before it listens) and the owner of the runtime mutation lock (the
 * process driving an update, a restart or a start). Windows keeps both beside the locked files.
 */
export async function recordedRuntimePids(
  home: string,
  platform: NodeJS.Platform = process.platform
): Promise<number[]> {
  const beside = platform === "win32";
  const [daemon, mutation] = await Promise.all([
    readText(join(home, beside ? "daemon.lock.pid" : "daemon.lock")),
    readText(join(home, beside ? "update.lock.owner" : "update.lock")),
  ]);
  const pids: number[] = [];
  if (/^\d+$/u.test(daemon)) pids.push(Number(daemon));
  try {
    const owner: unknown = mutation ? JSON.parse(mutation) : null;
    const pid = (owner as { pid?: unknown } | null)?.pid;
    if (typeof pid === "number") pids.push(pid);
  } catch {
    // An owner payload mid-write names no process yet.
  }
  return pids;
}

/**
 * Whether a runtime process is still alive: the one the product window is bound to, a replacement
 * that already holds the daemon lock, or the process restarting or updating the runtime.
 */
export async function runtimeProcessAlive(
  home: string,
  boundPid: number | undefined,
  alive: (pid: number) => boolean = processAlive
): Promise<boolean> {
  const pids = await recordedRuntimePids(home);
  if (boundPid !== undefined) pids.push(boundPid);
  return pids.some(pid => alive(pid));
}
