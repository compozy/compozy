import { execFile } from "node:child_process";
import { mkdir, mkdtemp, realpath, rm, symlink, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const run = promisify(execFile);

/**
 * Real git repositories for worktree E2E. The daemon shells out to git, so the
 * only honest fixture is a real repo — a stubbed one would pass while the
 * production path stays unexercised.
 */
export interface WorktreeRepoFixture {
  /** Repository root, registered as the workspace. */
  rootDir: string;
  /** Plain directory used to prove git-only controls stay absent. */
  nonGitDir: string;
  /** Creates an external checkout the daemon should discover but not own. */
  addExternalWorktree(name: string, branch: string): Promise<string>;
  /** Leaves uncommitted changes so removal refuses with real counts. */
  dirtyWorktree(worktreePath: string): Promise<void>;
  /** Removes a checkout behind the daemon's back to produce a missing record. */
  removeDirectory(target: string): Promise<void>;
  /** Re-creates a checkout at the exact recorded path so restore can revalidate. */
  restoreWorktreeAt(target: string, branch: string): Promise<void>;
  /** Makes setup deliberately observable so pending/cancel behavior is deterministic. */
  setSetupCommand(command: string): Promise<void>;
  /** Replaces one checkout with a directory symlink without touching its destination. */
  replaceDirectoryWithSymlink(target: string, destination: string): Promise<void>;
  /** Resolves symlinks for an untouched-directory assertion. */
  realPath(target: string): Promise<string>;
  /** Adds a GitHub-shaped origin while keeping all git traffic local. */
  setCredentiallessGitHubOrigin(): Promise<void>;
  /** Gives a worktree a published branch without contacting the fake remote URL. */
  publishWorktreeForBrowser(worktreePath: string): Promise<void>;
  cleanup(): Promise<void>;
}

async function git(cwd: string, ...args: string[]): Promise<string> {
  const { stdout } = await run("git", args, {
    cwd,
    env: {
      ...process.env,
      // Deterministic identity and no operator config bleeding into the fixture.
      GIT_AUTHOR_NAME: "CompozyOS E2E",
      GIT_AUTHOR_EMAIL: "e2e@compozy.test",
      GIT_COMMITTER_NAME: "CompozyOS E2E",
      GIT_COMMITTER_EMAIL: "e2e@compozy.test",
      GIT_CONFIG_GLOBAL: "/dev/null",
      GIT_CONFIG_SYSTEM: "/dev/null",
    },
  });
  return stdout;
}

/**
 * Explicit status assertion for daemon calls the specs use to prepare or verify
 * state. It never swallows an error: a 204 resolves with `null`, anything
 * outside the expected set throws with the payload.
 */
export async function expectDaemonStatus(
  call: Promise<unknown>,
  expected: { ok: true } | { failsWith: string }
): Promise<string | null> {
  try {
    await call;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    // A 204 has no body, so the JSON parse — not the status — is what threw.
    if ("ok" in expected && message.includes("invalid JSON")) return null;
    if ("failsWith" in expected && message.includes(expected.failsWith)) return message;
    throw new Error(`unexpected daemon response: ${message}`);
  }
  if ("failsWith" in expected) {
    throw new Error(`expected the daemon to refuse with ${expected.failsWith}, but it succeeded`);
  }
  return null;
}

export async function createWorktreeRepo(): Promise<WorktreeRepoFixture> {
  const baseDir = await mkdtemp(path.join(os.tmpdir(), "compozy-e2e-worktree-"));
  const rootDir = path.join(baseDir, "repo");
  const nonGitDir = path.join(baseDir, "plain-workspace");

  await run("mkdir", ["-p", rootDir]);
  await mkdir(nonGitDir, { recursive: true });
  await git(rootDir, "init", "--initial-branch=main");
  await writeFile(path.join(rootDir, "README.md"), "# worktree e2e fixture\n", "utf8");
  await git(rootDir, "add", "README.md");
  // A repo with zero commits is refused by the daemon, so seed one.
  await git(rootDir, "commit", "-m", "initial commit");

  return {
    rootDir,
    nonGitDir,
    async addExternalWorktree(name, branch) {
      const externalPath = path.join(baseDir, name);
      await git(rootDir, "worktree", "add", "-b", branch, externalPath);
      return externalPath;
    },
    async dirtyWorktree(worktreePath) {
      await writeFile(path.join(worktreePath, "uncommitted.txt"), "work in progress\n", "utf8");
    },
    async removeDirectory(target) {
      await rm(target, { recursive: true, force: true });
    },
    async restoreWorktreeAt(target, branch) {
      // git still holds the stale administrative entry, so prune it before
      // re-adding the same path.
      await git(rootDir, "worktree", "prune");
      await git(rootDir, "worktree", "add", target, branch);
    },
    async setSetupCommand(command) {
      const configDir = path.join(rootDir, ".compozy");
      await mkdir(configDir, { recursive: true });
      await writeFile(
        path.join(configDir, "config.toml"),
        `[worktrees]\nsetup_command = ${JSON.stringify(command)}\n`,
        "utf8"
      );
    },
    async replaceDirectoryWithSymlink(target, destination) {
      await rm(target, { recursive: true, force: true });
      await symlink(destination, target, "dir");
    },
    async realPath(target) {
      return realpath(target);
    },
    async setCredentiallessGitHubOrigin() {
      const remoteDir = path.join(baseDir, "origin.git");
      await git(baseDir, "init", "--bare", remoteDir);
      await git(rootDir, "remote", "add", "origin", remoteDir);
      await git(rootDir, "push", "-u", "origin", "main");
      await git(
        rootDir,
        "remote",
        "set-url",
        "origin",
        "https://github.com/compozy/e2e-fixture.git"
      );
    },
    async publishWorktreeForBrowser(worktreePath) {
      await writeFile(path.join(worktreePath, "published.txt"), "published change\n", "utf8");
      await git(worktreePath, "add", "published.txt");
      await git(worktreePath, "commit", "-m", "publish browser comparison fixture");
      const branch = (await git(worktreePath, "branch", "--show-current")).trim();
      await git(worktreePath, "update-ref", `refs/remotes/origin/${branch}`, "HEAD");
      await git(worktreePath, "branch", "--set-upstream-to", `origin/${branch}`);
    },
    async cleanup() {
      await rm(baseDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    },
  };
}
