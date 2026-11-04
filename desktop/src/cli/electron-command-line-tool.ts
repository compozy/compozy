import {
  access,
  constants,
  lstat,
  mkdir,
  readFile,
  realpath,
  stat,
  symlink,
} from "node:fs/promises";
import { homedir, userInfo } from "node:os";

import { app, dialog } from "electron";

import { writeFileAtomic } from "../files/atomic-write";
import {
  isExecutableFile,
  resolveLoginShellPath,
  runBoundedCommand,
} from "../product/login-shell-path";
import { ensureCommandLineTool, linkWithAdministratorPrivileges } from "./command-line-tool";
import { exposeCommandLineToolOnFirstRun, installCommandLineTool } from "./command-line-tool-flow";
import type { DesktopPaths } from "../home";
import type { DesktopLogger } from "../logging/desktop-logger";
import type { CommandLineToolFlowDeps } from "./command-line-tool-flow";

/** The PATH launchd gives a Finder-launched app, so the login shell rebuilds a fresh terminal's PATH. */
const LAUNCHD_PATH = "/usr/bin:/bin:/usr/sbin:/sbin";

function accountShell(): string | null {
  try {
    return userInfo().shell;
  } catch {
    return null;
  }
}

async function isWritableDirectory(path: string): Promise<boolean> {
  try {
    await access(path, constants.W_OK);
    return (await stat(path)).isDirectory();
  } catch {
    return false;
  }
}

async function isFile(path: string): Promise<boolean> {
  try {
    return (await stat(path)).isFile();
  } catch {
    return false;
  }
}

/** Wires the install flows to the real filesystem, login shell, macOS prompt, and dialogs. */
export function createCommandLineToolFlowDeps(options: {
  runtime: string;
  marker: string;
  log: (message: string, fields: Record<string, unknown>) => void;
}): CommandLineToolFlowDeps {
  return {
    ensure: async () =>
      await ensureCommandLineTool({
        runtime: options.runtime,
        home: homedir(),
        loginShellPath: async () =>
          await resolveLoginShellPath({
            env: { ...process.env, PATH: LAUNCHD_PATH },
            accountShell,
            run: runBoundedCommand,
            isExecutable: isExecutableFile,
            defaultShell: "/bin/zsh",
          }),
        isExecutable: isExecutableFile,
        fs: {
          isFile,
          exists: async path => {
            try {
              await lstat(path);
              return true;
            } catch (error) {
              if (error instanceof Error && "code" in error && error.code === "ENOENT")
                return false;
              throw error;
            }
          },
          realpath,
          isWritableDirectory,
          mkdir: async path => {
            await mkdir(path, { recursive: true, mode: 0o755 });
          },
          symlink: async (target, path) => await symlink(target, path),
        },
      }),
    linkWithAdministratorPrivileges: async path =>
      await linkWithAdministratorPrivileges(runBoundedCommand, options.runtime, path),
    showMessageBox: async messageBox => await dialog.showMessageBox(messageBox),
    attempted: async () => {
      try {
        await readFile(options.marker);
        return true;
      } catch {
        return false;
      }
    },
    recordAttempt: async state =>
      await writeFileAtomic(
        options.marker,
        `${JSON.stringify({ status: state.status, recorded_at: new Date().toISOString() })}\n`,
        0o600
      ),
    log: options.log,
  };
}

/** Keeps menu and automatic attempts single-flight and outside the bootstrap lifecycle. */
export function createDesktopCommandLineTool(
  paths: DesktopPaths,
  logger: Pick<DesktopLogger, "info" | "error">,
  e2e: boolean
): { install: () => void; firstRun: () => void } | null {
  if (process.platform !== "darwin" || e2e) return null;
  const deps = createCommandLineToolFlowDeps({
    runtime: paths.runtime,
    marker: paths.commandLineToolMarker,
    log: (message, fields) => logger.info(message, fields),
  });
  let pending: Promise<void> | null = null;
  const run = (automatic: boolean): void => {
    if (pending) {
      if (!automatic) {
        void dialog
          .showMessageBox({
            type: "info",
            message: "Command installation is already running",
            detail:
              "Finish the current installation check or approval prompt, then try again if needed.",
          })
          .catch(error => logger.error("show command installation status", error));
      }
      return;
    }
    if (automatic && (!app.isPackaged || process.env.COMPOZY_HOME?.trim())) return;
    pending = (automatic ? exposeCommandLineToolOnFirstRun(deps) : installCommandLineTool(deps))
      .catch(async error => {
        logger.error("install compozy command", error);
        if (!automatic) {
          await dialog.showMessageBox({
            type: "warning",
            message: "The compozy command was not installed",
            detail:
              "Check access to your terminal's command directory, then try again. See the desktop log for details.",
          });
        }
      })
      .finally(() => {
        pending = null;
      });
  };
  return { install: () => run(false), firstRun: () => run(true) };
}
