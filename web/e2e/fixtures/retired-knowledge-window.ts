import { execFile } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { CompozyApiOkJsonResponseFor } from "../../src/storybook/openapi-msw";
import type { BrowserRuntime } from "./runtime";
import { buildLaunchRuntimeEnv, prependPath } from "./runtime-helpers";
import { stopRegisteredDaemon } from "./runtime-process";

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

export type RetiredKnowledgeWindowSnapshot = CompozyApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager"
>;

export interface RetiredKnowledgeWindowSeed {
  workspaceId: string;
  sessionWindowId: string;
  knowledgeWindowId: string;
  profileId?: string;
  rect?: { x: number; y: number; width: number; height: number };
}

export async function seedRetiredKnowledgeWindow(
  runtime: BrowserRuntime,
  seed: RetiredKnowledgeWindowSeed
): Promise<RetiredKnowledgeWindowSnapshot> {
  if (runtime.mode !== "launch" || runtime.paths === undefined) {
    throw new Error("retired Knowledge window seeding requires a launch-mode isolated runtime");
  }

  // Client state has one writer. Seed the retired topology only after the
  // isolated daemon releases its store, then let the caller inspect raw state.
  await stopRegisteredDaemon({
    cliShim: runtime.paths.cliShim,
    homeDir: runtime.paths.homeDir,
    operatorHomeDir: runtime.paths.operatorHomeDir,
    repoRoot,
  });

  const args = [
    "run",
    "./web/e2e/fixtures/retired-knowledge-window-seeder",
    "--home",
    runtime.paths.homeDir,
    "--workspace",
    seed.workspaceId,
    "--session-window",
    seed.sessionWindowId,
    "--knowledge-window",
    seed.knowledgeWindowId,
  ];
  if (seed.profileId !== undefined) {
    args.push("--profile", seed.profileId);
  }
  if (seed.rect !== undefined) {
    args.push(
      "--x",
      String(seed.rect.x),
      "--y",
      String(seed.rect.y),
      "--width",
      String(seed.rect.width),
      "--height",
      String(seed.rect.height)
    );
  }
  const { stdout } = await execFileAsync("go", args, {
    cwd: repoRoot,
    maxBuffer: 16 * 1024 * 1024,
  });
  return JSON.parse(stdout) as RetiredKnowledgeWindowSnapshot;
}

export async function restartRetiredKnowledgeWindowRuntime(runtime: BrowserRuntime): Promise<void> {
  if (runtime.mode !== "launch" || runtime.paths === undefined) {
    throw new Error("retired Knowledge window restart requires a launch-mode isolated runtime");
  }
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
}
