import {
  spawn,
  type ChildProcessWithoutNullStreams,
  type SpawnOptionsWithoutStdio,
} from "node:child_process";

export interface CommandResult {
  readonly exitCode: number;
  readonly stderr: string;
  readonly stdout: string;
}

export interface RunningCommand {
  readonly child: ChildProcessWithoutNullStreams;
  readonly result: Promise<CommandResult>;
}

export function startCommand(
  executable: string,
  arguments_: readonly string[],
  options: SpawnOptionsWithoutStdio = {}
): RunningCommand {
  const child = spawn(executable, [...arguments_], { ...options, stdio: "pipe" });
  child.stderr.setEncoding("utf8");
  child.stdout.setEncoding("utf8");
  let stderr = "";
  let stdout = "";
  child.stderr.on("data", chunk => {
    stderr += String(chunk);
  });
  child.stdout.on("data", chunk => {
    stdout += String(chunk);
  });
  const result = new Promise<CommandResult>((resolveResult, reject) => {
    child.once("error", reject);
    child.once("close", exitCode => {
      resolveResult({ exitCode: exitCode ?? -1, stderr, stdout });
    });
  });
  return { child, result };
}

export async function runCommand(
  executable: string,
  arguments_: readonly string[],
  options: SpawnOptionsWithoutStdio = {}
): Promise<CommandResult> {
  return await startCommand(executable, arguments_, options).result;
}
