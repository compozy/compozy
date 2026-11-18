import { execFile } from "node:child_process";
import { readFile, stat } from "node:fs/promises";
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

/** A runtime pid and the moment (epoch ms) by which the runtime process holding it existed. */
export interface RuntimeProcessRecord {
  readonly pid: number;
  readonly recordedAt: number;
}

const START_LOOKUP_TIMEOUT_MS = 5_000;

function commandOutput(file: string, args: readonly string[]): Promise<string> {
  return new Promise(resolve => {
    execFile(
      file,
      [...args],
      {
        timeout: START_LOOKUP_TIMEOUT_MS,
        windowsHide: true,
        env: { ...process.env, LC_ALL: "C" },
      },
      (error, stdout) => resolve(error ? "" : stdout.trim())
    );
  });
}

/** When the process holding this pid started (epoch ms), or null when that cannot be read. */
export async function processStartedAt(
  pid: number,
  platform: NodeJS.Platform = process.platform,
  run: (file: string, args: readonly string[]) => Promise<string> = commandOutput
): Promise<number | null> {
  if (!Number.isSafeInteger(pid) || pid < 1) return null;
  // `ps` prints local time as "Thu Nov 12 10:00:00 2026"; PowerShell prints ISO 8601 in UTC.
  const text =
    platform === "win32"
      ? await run("powershell.exe", [
          "-NoProfile",
          "-NonInteractive",
          "-Command",
          `(Get-Process -Id ${pid}).StartTime.ToUniversalTime().ToString('o')`,
        ])
      : (await run("ps", ["-o", "lstart=", "-p", String(pid)])).replace(/\s+/gu, " ");
  const startedAt = text === "" ? Number.NaN : Date.parse(text);
  return Number.isNaN(startedAt) ? null : startedAt;
}

/** A record's text and last write, read in that order so the write never predates the text. */
async function readRecord(path: string): Promise<{ text: string; writtenAt: number }> {
  try {
    const text = (await readFile(path, "utf8")).trim();
    return { text, writtenAt: (await stat(path)).mtimeMs };
  } catch {
    return { text: "", writtenAt: 0 };
  }
}

/**
 * The processes the runtime records in its home while it runs or changes: the daemon lock holder
 * (a daemon records itself there before it listens) and the owner of the runtime mutation lock
 * (the process driving an update, a restart or a start). Each wrote its own pid, so it existed
 * when the file was last written. Windows keeps both beside the locked files.
 */
export async function recordedRuntimeProcesses(
  home: string,
  platform: NodeJS.Platform = process.platform
): Promise<RuntimeProcessRecord[]> {
  const beside = platform === "win32";
  const [daemon, mutation] = await Promise.all([
    readRecord(join(home, beside ? "daemon.lock.pid" : "daemon.lock")),
    readRecord(join(home, beside ? "update.lock.owner" : "update.lock")),
  ]);
  const records: RuntimeProcessRecord[] = [];
  if (/^\d+$/u.test(daemon.text)) {
    records.push({ pid: Number(daemon.text), recordedAt: daemon.writtenAt });
  }
  try {
    const owner: unknown = mutation.text ? JSON.parse(mutation.text) : null;
    const pid = (owner as { pid?: unknown } | null)?.pid;
    if (typeof pid === "number") records.push({ pid, recordedAt: mutation.writtenAt });
  } catch {
    // An owner payload mid-write names no process yet.
  }
  return records;
}

/**
 * Whether a runtime process is still alive: the one the product window is bound to, a replacement
 * that already holds the daemon lock, or the process restarting or updating the runtime.
 *
 * A pid outlives its process: a crashed daemon leaves its pid in the lock file and the system may
 * hand that pid to an unrelated program. So a pid counts only while its process started no later
 * than the moment the runtime was recorded under it; a later start is a reused pid, and a start
 * time that cannot be read confirms nothing.
 */
export async function runtimeProcessAlive(
  home: string,
  bound: RuntimeProcessRecord | undefined,
  probe: {
    readonly alive?: (pid: number) => boolean;
    readonly startedAt?: (pid: number) => Promise<number | null>;
  } = {}
): Promise<boolean> {
  const alive = probe.alive ?? processAlive;
  const startedAt = probe.startedAt ?? processStartedAt;
  const records = await recordedRuntimeProcesses(home);
  if (bound !== undefined) records.push(bound);
  for (const record of records) {
    if (!alive(record.pid)) continue;
    const started = await startedAt(record.pid);
    // A process starts before it is recorded and start times only round down, so no skew is
    // allowed: any start after the recorded moment is another program holding a reused pid.
    if (started !== null && started <= record.recordedAt) return true;
  }
  return false;
}
