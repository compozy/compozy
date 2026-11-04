import type { MessageBoxOptions } from "electron";

import type { CommandLineToolState } from "./command-line-tool";

/** Boundaries of the install flows: the decision, the privileged link, dialogs, and the marker. */
export interface CommandLineToolFlowDeps {
  readonly ensure: () => Promise<CommandLineToolState>;
  readonly linkWithAdministratorPrivileges: (path: string) => Promise<void>;
  readonly showMessageBox: (options: MessageBoxOptions) => Promise<{ response: number }>;
  /** True once the automatic first-run attempt has been recorded. */
  readonly attempted: () => Promise<boolean>;
  readonly recordAttempt: (state: CommandLineToolState) => Promise<void>;
  readonly log: (message: string, fields: Record<string, unknown>) => void;
}

const INSTALL_BUTTON = 0;

const readyDetail = "Open a new terminal window and run compozy --version.";

function resultDialog(state: CommandLineToolState): MessageBoxOptions {
  switch (state.status) {
    case "linked":
    case "current":
      return {
        type: "info",
        message: "The compozy command is ready",
        detail: `${state.path} runs this app's CompozyOS runtime. ${readyDetail}`,
      };
    case "existing":
      return {
        type: "info",
        message: "A compozy command is already installed",
        detail: `Your terminal runs ${state.path}. CompozyOS left it unchanged.`,
      };
    case "conflict":
      return {
        type: "warning",
        message: "An existing file blocks the compozy command",
        detail: `${state.path} is not an executable command. CompozyOS left it unchanged. Move or repair that file, then choose Install compozy Command again.`,
      };
    case "directory_unavailable":
      return {
        type: "warning",
        message: "The command directory is not available",
        detail: `CompozyOS could not create or write to ${state.path}. Check that it is a directory your account can write to, then choose Install compozy Command again.`,
      };
    case "install_failed":
      return {
        type: "warning",
        message: "The compozy command was not installed",
        detail:
          "Installation failed or was cancelled. You can try again from the CompozyOS menu. See the desktop log for details.",
      };
    case "needs_admin":
      return {
        type: "warning",
        message: "The compozy command was not installed",
        detail: `CompozyOS needs administrator access to create ${state.path}.`,
      };
    case "path_required":
      return {
        type: "warning",
        message: "The compozy command needs a terminal PATH",
        detail: `Add ${state.path} to your shell's PATH, then choose Install compozy Command again. No shell profiles were changed.`,
      };
    case "no_runtime":
      return {
        type: "info",
        message: "This app uses a CompozyOS runtime it did not install",
        detail: "Use the compozy command from that installation.",
      };
  }
}

async function checkCommandLineTool(deps: CommandLineToolFlowDeps): Promise<CommandLineToolState> {
  try {
    return await deps.ensure();
  } catch (error) {
    deps.log("check compozy command", {
      error: error instanceof Error ? error.message : String(error),
    });
    return { status: "install_failed" };
  }
}

/** Runs the administrator prompt for a link the user asked for, then reports the final state. */
async function linkAsAdministrator(
  deps: CommandLineToolFlowDeps,
  path: string
): Promise<CommandLineToolState> {
  try {
    await deps.linkWithAdministratorPrivileges(path);
  } catch (error) {
    deps.log("install compozy command with administrator access", {
      error: error instanceof Error ? error.message : String(error),
    });
  }
  const state = await checkCommandLineTool(deps);
  return state.status === "needs_admin" ? { status: "install_failed" } : state;
}

/** The app-menu action: always explicit, so it may raise the administrator prompt directly. */
export async function installCommandLineTool(deps: CommandLineToolFlowDeps): Promise<void> {
  let state = await checkCommandLineTool(deps);
  if (state.status === "needs_admin") state = await linkAsAdministrator(deps, state.path);
  deps.log("install compozy command", { status: state.status });
  await deps.showMessageBox(resultDialog(state));
}

/**
 * The first-run attempt: links silently when no privilege is needed, and otherwise asks once
 * before the administrator prompt. It runs once per CompozyOS home; the app menu reruns it.
 */
export async function exposeCommandLineToolOnFirstRun(
  deps: CommandLineToolFlowDeps
): Promise<void> {
  if (await deps.attempted()) return;
  const state = await checkCommandLineTool(deps);
  deps.log("expose compozy command on first run", { status: state.status });
  if (state.status === "no_runtime") return;
  await deps.recordAttempt(state);
  if (state.status === "linked" || state.status === "current") return;
  if (state.status !== "needs_admin") {
    await deps.showMessageBox(resultDialog(state));
    return;
  }
  const { response } = await deps.showMessageBox({
    type: "question",
    buttons: ["Install", "Not Now"],
    defaultId: INSTALL_BUTTON,
    cancelId: 1,
    message: "Use CompozyOS from your terminal?",
    detail:
      "CompozyOS can install the compozy command so a new terminal window can run it. " +
      "macOS asks for an administrator password. You can do this later from the CompozyOS menu.",
  });
  if (response !== INSTALL_BUTTON) return;
  await deps.showMessageBox(resultDialog(await linkAsAdministrator(deps, state.path)));
}
