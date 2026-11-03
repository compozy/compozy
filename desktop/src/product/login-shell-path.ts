import { spawn } from "node:child_process";
import { access, constants, stat } from "node:fs/promises";
import { posix } from "node:path";

/** Bounds the login-shell PATH resolution; mirrors `internal/procutil/login_shell_path.go`. */
export const LOGIN_SHELL_PATH_TIMEOUT_MS = 5_000;
const LOGIN_SHELL_PATH_MAX_BYTES = 64 * 1024;

export interface RunOptions {
  cwd?: string;
  env: NodeJS.ProcessEnv;
  timeoutMs: number;
  maxBytes: number;
}

/** Runs a program without a shell and resolves its stdout; rejects on failure or timeout. */
export type CommandRunner = (file: string, args: string[], options: RunOptions) => Promise<string>;

export interface LoginShellPathDeps {
  env: NodeJS.ProcessEnv;
  /** The account's recorded login shell (`os.userInfo().shell`), when known. */
  accountShell: () => string | null;
  run: CommandRunner;
  isExecutable: (path: string) => Promise<boolean>;
  defaultShell: string;
}

/** Extracts the PATH printed between the last pair of NUL delimiters. */
export function parseLoginShellOutput(output: string): string | null {
  const end = output.lastIndexOf("\0");
  if (end < 0) return null;
  const start = output.lastIndexOf("\0", end - 1);
  if (start < 0) return null;
  const path = output.slice(start + 1, end);
  if (path.trim() === "" || /[\0\r\n]/.test(path)) return null;
  return path;
}

/**
 * The PATH the operator's login shell produces: the inherited `SHELL` when it names an
 * executable, else the account shell, else the platform default. Resolves `null` on any
 * failure so callers keep the process PATH.
 */
export async function resolveLoginShellPath(deps: LoginShellPathDeps): Promise<string | null> {
  const home = deps.env.HOME?.trim();
  if (!home || !posix.isAbsolute(home)) return null;
  const inherited = deps.env.SHELL?.trim();
  const account = deps.accountShell()?.trim();
  let shell = deps.defaultShell;
  if (inherited && posix.isAbsolute(inherited) && (await deps.isExecutable(inherited))) {
    shell = inherited;
  } else if (account && posix.isAbsolute(account)) {
    shell = account;
  }
  try {
    const output = await deps.run(shell, ["-ilc", `printf '\\000%s\\000' "$PATH"`], {
      cwd: home,
      env: { ...deps.env, DISABLE_AUTO_UPDATE: "true" },
      timeoutMs: LOGIN_SHELL_PATH_TIMEOUT_MS,
      maxBytes: LOGIN_SHELL_PATH_MAX_BYTES,
    });
    return parseLoginShellOutput(output);
  } catch {
    return null;
  }
}

/** Resolves a bare executable name against a POSIX PATH value, or `null` when absent. */
export async function lookPath(
  name: string,
  pathValue: string,
  isExecutable: (path: string) => Promise<boolean>
): Promise<string | null> {
  for (const directory of pathValue.split(":")) {
    if (!posix.isAbsolute(directory)) continue;
    const candidate = posix.join(directory, name);
    if (await isExecutable(candidate)) return candidate;
  }
  return null;
}

/** True for an executable regular file. */
export async function isExecutableFile(path: string): Promise<boolean> {
  try {
    await access(path, constants.X_OK);
    return (await stat(path)).isFile();
  } catch {
    return false;
  }
}

/**
 * Spawns `file` in its own process group with no shell, collects bounded stdout, and kills
 * the whole group when the deadline passes (an interactive login shell may start helpers).
 */
export const runBoundedCommand: CommandRunner = async (file, args, options) =>
  await new Promise<string>((resolve, reject) => {
    const child = spawn(file, args, {
      cwd: options.cwd,
      env: options.env,
      shell: false,
      detached: process.platform !== "win32",
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
    });
    const chunks: Buffer[] = [];
    let size = 0;
    let failure: Error | null = null;
    const kill = (reason: Error): void => {
      failure ??= reason;
      try {
        if (child.pid && process.platform !== "win32") process.kill(-child.pid, "SIGKILL");
        else child.kill("SIGKILL");
      } catch {
        // The group already exited.
      }
    };
    const deadline = setTimeout(
      () => kill(new Error(`${file} timed out after ${options.timeoutMs}ms.`)),
      options.timeoutMs
    );
    child.stdout?.on("data", (chunk: Buffer) => {
      size += chunk.length;
      if (size > options.maxBytes) kill(new Error(`${file} output exceeded the limit.`));
      else chunks.push(chunk);
    });
    child.once("error", error => {
      clearTimeout(deadline);
      reject(error);
    });
    child.once("close", code => {
      clearTimeout(deadline);
      if (failure) reject(failure);
      else if (code !== 0) reject(new Error(`${file} exited with status ${code}.`));
      else resolve(Buffer.concat(chunks).toString("utf8"));
    });
  });
