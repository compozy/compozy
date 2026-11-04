import { posix } from "node:path";

import type { CommandRunner } from "../product/login-shell-path";

/** The command name every CompozyOS install exposes. */
export const COMMAND_NAME = "compozy";
/** The installer's preferred directory (`scripts/install.template.sh`); on the default macOS PATH. */
export const SYSTEM_BIN_DIRECTORY = "/usr/local/bin";
/** How long the macOS administrator prompt may stay open before the request is abandoned. */
export const ADMINISTRATOR_PROMPT_TIMEOUT_MS = 5 * 60_000;

/**
 * Where the terminal `compozy` command stands relative to the desktop-installed runtime.
 * `linked` and `current` both mean a fresh terminal runs the desktop runtime.
 */
export type CommandLineToolState =
  | { readonly status: "linked"; readonly path: string }
  | { readonly status: "current"; readonly path: string }
  | { readonly status: "existing"; readonly path: string }
  | { readonly status: "needs_admin"; readonly path: string }
  | { readonly status: "path_required"; readonly path: string }
  | { readonly status: "no_runtime" };

/** Filesystem and environment boundaries, injected so the decision logic is testable. */
export interface CommandLineToolDeps {
  /** The stable desktop runtime path, `$COMPOZY_HOME/bin/compozy`. */
  readonly runtime: string;
  /** The operator's home directory, for the installer's `~/.local/bin` fallback. */
  readonly home: string;
  /** The operator's login-shell PATH, or `null` when it cannot be resolved. */
  readonly loginShellPath: () => Promise<string | null>;
  readonly isExecutable: (path: string) => Promise<boolean>;
  readonly fs: {
    isFile(path: string): Promise<boolean>;
    exists(path: string): Promise<boolean>;
    realpath(path: string): Promise<string>;
    isWritableDirectory(path: string): Promise<boolean>;
    mkdir(path: string): Promise<void>;
    symlink(target: string, path: string): Promise<void>;
  };
}

function pathDirectories(pathValue: string | null): string[] {
  if (!pathValue) return [];
  return pathValue
    .split(":")
    .filter(directory => posix.isAbsolute(directory))
    .map(directory => posix.normalize(directory).replace(/(.)\/$/, "$1"));
}

async function sameFile(deps: CommandLineToolDeps, left: string, right: string): Promise<boolean> {
  try {
    return (await deps.fs.realpath(left)) === (await deps.fs.realpath(right));
  } catch {
    return false;
  }
}

/** The command a fresh terminal would run, classified against the desktop runtime. */
async function firstCommandOnPath(
  deps: CommandLineToolDeps,
  directories: readonly string[]
): Promise<CommandLineToolState | null> {
  for (const directory of directories) {
    const candidate = posix.join(directory, COMMAND_NAME);
    if (!(await deps.isExecutable(candidate))) continue;
    return (await sameFile(deps, candidate, deps.runtime))
      ? { status: "current", path: candidate }
      : { status: "existing", path: candidate };
  }
  return null;
}

/**
 * Makes `compozy` resolvable from a fresh terminal by linking the installer's directory to the
 * stable desktop runtime path, so runtime upgrades keep the link valid. It never replaces an
 * existing command: the first `compozy` on the login PATH wins and is reported instead.
 */
export async function ensureCommandLineTool(
  deps: CommandLineToolDeps
): Promise<CommandLineToolState> {
  if (!(await deps.fs.isFile(deps.runtime))) return { status: "no_runtime" };
  const directories = pathDirectories(await deps.loginShellPath());
  const found = await firstCommandOnPath(deps, directories);
  if (found) return found;

  const systemLink = posix.join(SYSTEM_BIN_DIRECTORY, COMMAND_NAME);
  const userDirectory = posix.join(deps.home, ".local", "bin");
  let target: string | null = null;
  if (
    directories.includes(SYSTEM_BIN_DIRECTORY) &&
    (await deps.fs.isWritableDirectory(SYSTEM_BIN_DIRECTORY))
  ) {
    target = SYSTEM_BIN_DIRECTORY;
  } else if (directories.includes(userDirectory)) {
    await deps.fs.mkdir(userDirectory);
    if (await deps.fs.isWritableDirectory(userDirectory)) target = userDirectory;
  }
  if (!target) {
    if (directories.includes(SYSTEM_BIN_DIRECTORY) && (await deps.fs.exists(systemLink))) {
      return { status: "existing", path: systemLink };
    }
    return directories.includes(SYSTEM_BIN_DIRECTORY)
      ? { status: "needs_admin", path: systemLink }
      : { status: "path_required", path: userDirectory };
  }
  const link = posix.join(target, COMMAND_NAME);
  try {
    await deps.fs.symlink(deps.runtime, link);
  } catch (error) {
    if (!(error instanceof Error && "code" in error && error.code === "EEXIST")) throw error;
    return (await sameFile(deps, link, deps.runtime))
      ? { status: "current", path: link }
      : { status: "existing", path: link };
  }
  return { status: "linked", path: link };
}

/**
 * Creates the system link through the macOS administrator prompt. Paths travel as AppleScript
 * arguments and `quoted form of`, so no path is ever interpolated into a shell string, and
 * `ln -s` without `-f` refuses to replace anything created in the meantime.
 */
export async function linkWithAdministratorPrivileges(
  run: CommandRunner,
  runtime: string,
  link: string
): Promise<void> {
  await run(
    "/usr/bin/osascript",
    [
      "-e",
      "on run argv",
      "-e",
      'do shell script "/bin/mkdir -p " & quoted form of (item 2 of argv) & " && /bin/test ! -e " & quoted form of (item 3 of argv) & " && /bin/test ! -L " & quoted form of (item 3 of argv) & " && /bin/ln -sh " & quoted form of (item 1 of argv) & " " & quoted form of (item 3 of argv) with administrator privileges',
      "-e",
      "end run",
      runtime,
      posix.dirname(link),
      link,
    ],
    { env: { PATH: "/usr/bin:/bin" }, timeoutMs: ADMINISTRATOR_PROMPT_TIMEOUT_MS, maxBytes: 4096 }
  );
}
