import { execFile, type ChildProcessWithoutNullStreams } from "node:child_process";
import process from "node:process";
import { promisify } from "node:util";

const DAEMON_STOP_TIMEOUT_MS = 20_000;
const PROCESS_CLOSE_TIMEOUT_MS = 10_000;
const PROCESS_KILL_TIMEOUT_MS = 5_000;

interface RegisteredDaemonCleanup {
  cliShim: string;
  homeDir: string;
  repoRoot: string;
}

interface ExecuteFileResult {
  stderr: string;
  stdout: string;
}

type ExecuteFile = (
  file: string,
  args: readonly string[],
  options: { cwd: string; env: NodeJS.ProcessEnv; timeout: number }
) => Promise<ExecuteFileResult>;

interface BrowserDaemonStopDependencies {
  executeFile?: ExecuteFile;
  stopSpawned?: (child: ChildProcessWithoutNullStreams) => Promise<void>;
}

const execFileAsync = promisify(execFile);

async function executeFile(
  file: string,
  args: readonly string[],
  options: { cwd: string; env: NodeJS.ProcessEnv; timeout: number }
): Promise<ExecuteFileResult> {
  const result = await execFileAsync(file, [...args], options);
  return {
    stderr: String(result.stderr ?? ""),
    stdout: String(result.stdout ?? ""),
  };
}

async function stopRegisteredDaemon(
  cleanup: RegisteredDaemonCleanup,
  run: ExecuteFile = executeFile
): Promise<void> {
  try {
    await run(cleanup.cliShim, ["daemon", "stop", "-o", "json"], {
      cwd: cleanup.repoRoot,
      env: {
        ...process.env,
        AGH_HOME: cleanup.homeDir,
        HOME: cleanup.homeDir,
      },
      timeout: DAEMON_STOP_TIMEOUT_MS,
    });
  } catch (error) {
    const message = commandErrorText(error);
    if (message.includes("daemon is not running")) {
      return;
    }
    throw new Error(`browser runtime registered daemon stop failed: ${message}`, { cause: error });
  }
}

async function stopBrowserDaemonProcess(
  child: ChildProcessWithoutNullStreams,
  cleanup: RegisteredDaemonCleanup,
  dependencies: BrowserDaemonStopDependencies = {}
): Promise<void> {
  const errors: Error[] = [];
  try {
    await stopRegisteredDaemon(cleanup, dependencies.executeFile ?? executeFile);
  } catch (error) {
    errors.push(errorFromUnknown(error));
  }

  try {
    await (dependencies.stopSpawned ?? stopSpawnedDaemonProcess)(child);
  } catch (error) {
    errors.push(errorFromUnknown(error));
  }

  if (errors.length > 0) {
    throw new AggregateError(errors, "browser runtime daemon cleanup failed");
  }
}

async function stopSpawnedDaemonProcess(child: ChildProcessWithoutNullStreams): Promise<void> {
  if (child.exitCode !== null) {
    return;
  }

  child.kill("SIGINT");
  const closed = await waitForClose(child, PROCESS_CLOSE_TIMEOUT_MS);
  if (closed) {
    return;
  }

  child.kill("SIGKILL");
  if (!(await waitForClose(child, PROCESS_KILL_TIMEOUT_MS))) {
    throw new Error(`browser runtime daemon process ${child.pid ?? "unknown"} did not exit`);
  }
}

function waitForClose(child: ChildProcessWithoutNullStreams, timeoutMs: number): Promise<boolean> {
  return new Promise(resolve => {
    const timeout = setTimeout(() => {
      cleanup();
      resolve(false);
    }, timeoutMs);

    const handleClose = () => {
      cleanup();
      resolve(true);
    };

    const cleanup = () => {
      clearTimeout(timeout);
      child.off("close", handleClose);
    };

    child.once("close", handleClose);
  });
}

function commandErrorText(error: unknown): string {
  if (!(error instanceof Error)) {
    return String(error);
  }
  const output = error as Error & { stderr?: unknown; stdout?: unknown };
  return [error.message, output.stderr, output.stdout]
    .map(value => String(value ?? "").trim())
    .filter(Boolean)
    .join("\n");
}

function errorFromUnknown(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error));
}

export {
  stopBrowserDaemonProcess,
  stopRegisteredDaemon,
  stopSpawnedDaemonProcess,
  type BrowserDaemonStopDependencies,
  type ExecuteFile,
  type RegisteredDaemonCleanup,
};
