import { execFile } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { BrowserRuntime } from "./runtime";

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

export interface RetainedLoopTaskSeed {
  loopRunId: string;
  profileId: string;
  runId: string;
  taskId: string;
  workspaceId: string;
}

/**
 * Creates the E2E-020 retention boundary through the isolated runtime's SQLite
 * store: the task run retains structured loop_run_id/run_kind facts, while no
 * owning loop_runs row or loop_name metadata exists.
 *
 * Task tests can call this after resolving their workspace, then open the task
 * by taskId or request the catalog with include_loop=true.
 */
export async function seedRetainedLoopTask(
  runtime: BrowserRuntime,
  seed: RetainedLoopTaskSeed
): Promise<RetainedLoopTaskSeed> {
  if (runtime.mode !== "launch" || runtime.paths === undefined) {
    throw new Error("retained Loop task seeding requires a launch-mode isolated runtime");
  }

  await execFileAsync(
    "go",
    [
      "run",
      "./web/e2e/fixtures/retained-loop-task-seeder",
      "--home",
      runtime.paths.homeDir,
      "--workspace",
      seed.workspaceId,
      "--profile",
      seed.profileId,
      "--task",
      seed.taskId,
      "--run",
      seed.runId,
      "--loop-run",
      seed.loopRunId,
    ],
    { cwd: repoRoot, maxBuffer: 1024 * 1024 }
  );
  return seed;
}
