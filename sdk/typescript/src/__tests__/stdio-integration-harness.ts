import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import readline from "node:readline";

const messageTimeoutMs = 5_000;
export type RpcMessage = Record<string, unknown>;

interface ProcessDiagnostics {
  child: ChildProcessWithoutNullStreams;
  label: string;
  stderrLines?: string[];
}

function formatDiagnostics({ child, label, stderrLines = [] }: ProcessDiagnostics): string {
  const exitDetail =
    child.exitCode === null && child.signalCode === null
      ? "still running"
      : `exit=${child.exitCode ?? "null"} signal=${child.signalCode ?? "null"}`;
  const stderrTail = stderrLines.slice(-20).join("\n");
  return [
    `while waiting for ${label}`,
    `child ${exitDetail}`,
    stderrTail ? `stderr:\n${stderrTail}` : "stderr: <empty>",
  ].join("; ");
}

interface LineWaiter {
  accept: (line: string) => void;
}

class BufferedMessageReader {
  private readonly queuedLines: string[] = [];
  private readonly waiters: LineWaiter[] = [];
  private closed = false;

  public constructor(private readonly rl: readline.Interface) {
    this.rl.on("line", line => {
      const waiter = this.waiters.shift();
      if (waiter) {
        waiter.accept(line);
        return;
      }
      this.queuedLines.push(line);
    });
    this.rl.on("close", () => {
      this.closed = true;
    });
  }

  public async next(diagnostics: ProcessDiagnostics): Promise<Record<string, unknown>> {
    const line = await this.nextLine(diagnostics);
    try {
      return JSON.parse(line) as Record<string, unknown>;
    } catch (error) {
      throw error instanceof Error
        ? new Error(`invalid JSON ${formatDiagnostics(diagnostics)}: ${error.message}`)
        : new Error(`invalid JSON ${formatDiagnostics(diagnostics)}: ${String(error)}`);
    }
  }

  private async nextLine(diagnostics: ProcessDiagnostics): Promise<string> {
    const queued = this.queuedLines.shift();
    if (queued !== undefined) {
      return queued;
    }
    if (this.closed) {
      throw new Error(`stdout closed ${formatDiagnostics(diagnostics)}`);
    }

    return await new Promise((resolve, reject) => {
      const waiter: LineWaiter = {
        accept: line => {
          cleanup();
          resolve(line);
        },
      };
      const timeout = setTimeout(() => {
        cleanup();
        reject(new Error(`timed out ${formatDiagnostics(diagnostics)}`));
      }, messageTimeoutMs);

      const cleanup = (): void => {
        clearTimeout(timeout);
        const waiterIndex = this.waiters.indexOf(waiter);
        if (waiterIndex !== -1) {
          this.waiters.splice(waiterIndex, 1);
        }
        this.rl.off("close", onClose);
        diagnostics.child.off("exit", onExit);
        diagnostics.child.off("error", onError);
      };
      const onClose = (): void => {
        cleanup();
        reject(new Error(`stdout closed ${formatDiagnostics(diagnostics)}`));
      };
      const onExit = (): void => {
        cleanup();
        reject(new Error(`child exited ${formatDiagnostics(diagnostics)}`));
      };
      const onError = (error: Error): void => {
        cleanup();
        reject(
          new Error(`child process error ${formatDiagnostics(diagnostics)}: ${error.message}`)
        );
      };

      this.waiters.push(waiter);
      this.rl.once("close", onClose);
      diagnostics.child.once("exit", onExit);
      diagnostics.child.once("error", onError);
    });
  }
}

async function waitForStderrLine(
  rl: readline.Interface,
  diagnostics: ProcessDiagnostics,
  pattern: RegExp
): Promise<void> {
  if ((diagnostics.stderrLines ?? []).some(line => pattern.test(line))) {
    return;
  }

  await new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup();
      reject(new Error(`timed out ${formatDiagnostics(diagnostics)}`));
    }, messageTimeoutMs);

    const cleanup = (): void => {
      clearTimeout(timeout);
      rl.off("line", onLine);
      rl.off("close", onClose);
      diagnostics.child.off("exit", onExit);
      diagnostics.child.off("error", onError);
    };
    const onLine = (line: string): void => {
      if (!pattern.test(line)) {
        return;
      }
      cleanup();
      resolve();
    };
    const onClose = (): void => {
      cleanup();
      reject(new Error(`stderr closed ${formatDiagnostics(diagnostics)}`));
    };
    const onExit = (): void => {
      cleanup();
      reject(new Error(`child exited ${formatDiagnostics(diagnostics)}`));
    };
    const onError = (error: Error): void => {
      cleanup();
      reject(new Error(`child process error ${formatDiagnostics(diagnostics)}: ${error.message}`));
    };

    rl.on("line", onLine);
    rl.once("close", onClose);
    diagnostics.child.once("exit", onExit);
    diagnostics.child.once("error", onError);
  });
}

async function terminateChild(
  child: ChildProcessWithoutNullStreams,
  interfaces: readline.Interface[]
): Promise<void> {
  for (const rl of interfaces) {
    rl.close();
  }
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }

  const exited = new Promise<void>(resolve => {
    child.once("exit", () => resolve());
  });
  child.kill();
  await Promise.race([exited, new Promise(resolve => setTimeout(resolve, 1_000))]);
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }
  child.kill("SIGKILL");
  await Promise.race([exited, new Promise(resolve => setTimeout(resolve, 1_000))]);
}

type ExtensionProcess = {
  send(message: RpcMessage): void;
  read(label: string): Promise<RpcMessage>;
  waitForStderr(label: string, pattern: RegExp): Promise<void>;
};

/** Runs a fixture extension with a deadline and diagnostics for each stdio exchange. */
export async function withExtensionProcess<T>(
  source: string,
  run: (extension: ExtensionProcess) => Promise<T>
): Promise<T> {
  const directory = await mkdtemp(join(tmpdir(), "compozy-sdk-integration-"));
  try {
    const entry = join(directory, "index.mjs");
    await writeFile(entry, source);
    const child = spawn(process.execPath, [entry], { stdio: ["pipe", "pipe", "pipe"] });
    let stdinError: Error | undefined;
    child.stdin.on("error", error => {
      stdinError ??= error;
    });
    const stdoutInterface = readline.createInterface({ input: child.stdout });
    const stdout = new BufferedMessageReader(stdoutInterface);
    const stderr = readline.createInterface({ input: child.stderr });
    const stderrLines: string[] = [];
    stderr.on("line", line => stderrLines.push(line));

    try {
      return await run({
        send: message => {
          child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", ...message })}\n`);
        },
        read: label => {
          const diagnostics = { child, label, stderrLines };
          if (stdinError) {
            return Promise.reject(
              new Error(`stdin write failed ${formatDiagnostics(diagnostics)}: ${stdinError.message}`)
            );
          }
          return stdout.next(diagnostics);
        },
        waitForStderr: (label, pattern) =>
          waitForStderrLine(stderr, { child, label, stderrLines }, pattern),
      });
    } finally {
      await terminateChild(child, [stdoutInterface, stderr]);
    }
  } finally {
    await rm(directory, { force: true, recursive: true });
  }
}
