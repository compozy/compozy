import { spawn } from "node:child_process";
import { realpath, stat } from "node:fs/promises";
import { homedir, userInfo } from "node:os";

import { dialog } from "electron";

import { ElectronFileOpener, type FileOpener } from "./file-opener";
import { isExecutableFile, resolveLoginShellPath, runBoundedCommand } from "./login-shell-path";

function accountShell(): string | null {
  try {
    return userInfo().shell;
  } catch {
    return null;
  }
}

/** Wires the opener to Electron's native dialog and the real process, fs, and fetch boundaries. */
export function createElectronFileOpener(options: {
  origin: () => string | null;
  log: (message: string, fields: Record<string, unknown>) => void;
}): FileOpener {
  return new ElectronFileOpener({
    platform: process.platform,
    homedir,
    env: process.env,
    fs: { realpath, stat },
    showMessageBox: async (window, messageBox) =>
      window
        ? await dialog.showMessageBox(window, messageBox)
        : await dialog.showMessageBox(messageBox),
    origin: options.origin,
    fetch: async (input, init) => await fetch(input, init),
    spawn,
    run: runBoundedCommand,
    loginShellPath: async () =>
      await resolveLoginShellPath({
        env: process.env,
        accountShell,
        run: runBoundedCommand,
        isExecutable: isExecutableFile,
        defaultShell: process.platform === "darwin" ? "/bin/zsh" : "/bin/sh",
      }),
    isExecutable: isExecutableFile,
    log: options.log,
  });
}
