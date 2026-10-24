import { execFile } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { BrowserRuntime } from "./runtime";
import { buildLaunchRuntimeEnv, prependPath } from "./runtime-helpers";
import { stopRegisteredDaemon } from "./runtime-process";

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

export interface SessionsBoardSeed {
  workspaceId: string;
  agentName: string;
  prefix?: string;
  done?: number;
  archived?: number;
  /** A parent with three direct children; the first child failed (Needs you). */
  family?: boolean;
}

export interface SessionsBoardSeeded {
  done: string[];
  archived: string[];
  parent?: string;
  children?: string[];
}

/**
 * Writes durable board rows through the store while the isolated daemon is
 * stopped, then boots it again. Rows are stopped sessions, so the daemon's own
 * board-column derivation places them (Safety Invariant 11).
 */
export async function seedSessionsBoard(
  runtime: BrowserRuntime,
  seed: SessionsBoardSeed
): Promise<SessionsBoardSeeded> {
  if (runtime.mode !== "launch" || runtime.paths === undefined) {
    throw new Error("sessions board seeding requires a launch-mode isolated runtime");
  }
  await stopRegisteredDaemon({
    cliShim: runtime.paths.cliShim,
    homeDir: runtime.paths.homeDir,
    operatorHomeDir: runtime.paths.operatorHomeDir,
    repoRoot,
  });
  const args = [
    "run",
    "./web/e2e/fixtures/sessions-board-seeder",
    "--home",
    runtime.paths.homeDir,
    "--workspace",
    seed.workspaceId,
    "--agent",
    seed.agentName,
    "--prefix",
    seed.prefix ?? "sess-board",
    "--done",
    String(seed.done ?? 0),
    "--archived",
    String(seed.archived ?? 0),
  ];
  if (seed.family) args.push("--family");
  const { stdout } = await execFileAsync("go", args, { cwd: repoRoot, maxBuffer: 4 * 1024 * 1024 });
  await execFileAsync(runtime.paths.cliShim, ["daemon", "start", "-o", "json"], {
    cwd: repoRoot,
    env: buildLaunchRuntimeEnv(repoRoot, {
      ...process.env,
      COMPOZY_E2E_CLI_BIN: runtime.paths.cliShim,
      COMPOZY_HOME: runtime.paths.homeDir,
      HOME: runtime.paths.operatorHomeDir,
      PATH: prependPath(path.dirname(runtime.paths.cliShim), process.env.PATH),
    }),
    maxBuffer: 1024 * 1024,
    timeout: 90_000,
  });
  return JSON.parse(stdout) as SessionsBoardSeeded;
}
