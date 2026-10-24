import { execFile } from "node:child_process";
import { access, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { BrowserRuntime } from "./runtime";
import { buildLaunchRuntimeEnv, prependPath } from "./runtime-helpers";
import { stopRegisteredDaemon } from "./runtime-process";

const run = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

/** The origin every PR chip journey points the workspace at; git traffic stays local. */
export const PULL_REQUEST_REPO_KEY = "github.com/compozy/e2e-fixture";

export interface SessionPullRequestSeed {
  workspaceId: string;
  sessionId: string;
  branch: string;
  number: number;
  checks: "passing" | "failing";
  review: "approved" | "review_required" | "changes_requested";
}

async function git(cwd: string, ...args: string[]): Promise<void> {
  await run("git", args, {
    cwd,
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "CompozyOS E2E",
      GIT_AUTHOR_EMAIL: "e2e@compozy.test",
      GIT_COMMITTER_NAME: "CompozyOS E2E",
      GIT_COMMITTER_EMAIL: "e2e@compozy.test",
      GIT_CONFIG_GLOBAL: "/dev/null",
      GIT_CONFIG_SYSTEM: "/dev/null",
    },
  });
}

/** Makes the isolated workspace a real repository whose origin is GitHub-shaped. */
async function ensureGitHubOrigin(workspaceDir: string): Promise<void> {
  const isRepo = await access(path.join(workspaceDir, ".git")).then(
    () => true,
    () => false
  );
  if (isRepo) return;
  await git(workspaceDir, "init", "--initial-branch=main");
  await writeFile(path.join(workspaceDir, "README.md"), "# pr chip e2e fixture\n", "utf8");
  await git(workspaceDir, "add", "README.md");
  await git(workspaceDir, "commit", "-m", "initial commit");
  await git(workspaceDir, "remote", "add", "origin", `https://${PULL_REQUEST_REPO_KEY}.git`);
}

/**
 * Gives an existing workspace-root session its settled branch snapshot and the
 * cached PR for that branch through the store while the isolated daemon is
 * stopped, then boots it again. The daemon's own read path joins them.
 */
export async function seedSessionPullRequest(
  runtime: BrowserRuntime,
  seed: SessionPullRequestSeed
): Promise<void> {
  if (runtime.mode !== "launch" || runtime.paths === undefined) {
    throw new Error("pull request seeding requires a launch-mode isolated runtime");
  }
  await ensureGitHubOrigin(runtime.paths.workspaceDir);
  await stopRegisteredDaemon({
    cliShim: runtime.paths.cliShim,
    homeDir: runtime.paths.homeDir,
    operatorHomeDir: runtime.paths.operatorHomeDir,
    repoRoot,
  });
  await run(
    "go",
    [
      "run",
      "./web/e2e/fixtures/session-pull-request-seeder",
      "--home",
      runtime.paths.homeDir,
      "--workspace",
      seed.workspaceId,
      "--session",
      seed.sessionId,
      "--branch",
      seed.branch,
      "--repo-key",
      PULL_REQUEST_REPO_KEY,
      "--number",
      String(seed.number),
      "--checks",
      seed.checks,
      "--review",
      seed.review,
    ],
    { cwd: repoRoot, maxBuffer: 4 * 1024 * 1024 }
  );
  await run(runtime.paths.cliShim, ["daemon", "start", "-o", "json"], {
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
