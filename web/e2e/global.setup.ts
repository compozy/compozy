import { appendFile, cp, mkdir, rm, stat, writeFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import path from "node:path";

import type { FullConfig } from "@playwright/test";

import {
  PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG,
  PLAYWRIGHT_RUN_SEED_WORKFLOW_SLUG,
  PLAYWRIGHT_SOURCE_WORKFLOW_SLUGS,
  PLAYWRIGHT_START_WORKFLOW_SLUG,
  PLAYWRIGHT_WORKFLOW_SLUGS,
  resolvePlaywrightPaths,
  type DaemonUIEnvironment,
} from "./support/daemon-fixture";

interface CommandResult {
  stdout: string;
  stderr: string;
}

interface WorkspaceListPayload {
  workspaces?: Array<{
    id: string;
    name: string;
    root_dir: string;
  }>;
}

interface DaemonStartPayload {
  daemon?: {
    http_port?: number;
  };
}

export default async function globalSetup(_config: FullConfig): Promise<void> {
  const paths = resolvePlaywrightPaths();
  await rm(paths.sharedTmpDir, { recursive: true, force: true });
  await mkdir(paths.sharedTmpDir, { recursive: true });
  await assertBinaryExists(paths.binaryPath);

  const fixtureRoot = path.join(paths.sharedTmpDir, "workspace");
  const homeDir = path.join(paths.sharedTmpDir, "home");
  await createWorkspaceFixture(paths.repoRoot, fixtureRoot);
  await mkdir(homeDir, { recursive: true });

  const env = daemonEnvironment(homeDir);

  await runCLI(
    paths.commandLogFile,
    paths.binaryPath,
    ["setup", "--agent", "codex", "--global", "--yes"],
    {
      cwd: fixtureRoot,
      env,
    }
  );

  const start = await runCLI(
    paths.commandLogFile,
    paths.binaryPath,
    ["daemon", "start", "--format", "json"],
    {
      cwd: fixtureRoot,
      env: { ...env, COMPOZY_DAEMON_HTTP_PORT: "0" },
    }
  );
  const startPayload = parseJSON<DaemonStartPayload>(start.stdout, "daemon start");
  const port = startPayload.daemon?.http_port;
  if (!port || port <= 0) {
    throw new Error(`daemon start did not report a valid HTTP port:\n${start.stdout}`);
  }
  const baseUrl = `http://127.0.0.1:${port}`;

  await runCLI(
    paths.commandLogFile,
    paths.binaryPath,
    ["workspaces", "resolve", fixtureRoot, "--format", "json"],
    {
      cwd: fixtureRoot,
      env,
    }
  );

  for (const slug of [...PLAYWRIGHT_WORKFLOW_SLUGS, PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG]) {
    await runCLI(
      paths.commandLogFile,
      paths.binaryPath,
      ["sync", "--name", slug, "--format", "json"],
      {
        cwd: fixtureRoot,
        env,
      }
    );
  }

  const workspaceList = await runCLI(
    paths.commandLogFile,
    paths.binaryPath,
    ["workspaces", "list", "--format", "json"],
    {
      cwd: fixtureRoot,
      env,
    }
  );
  const workspacePayload = parseJSON<WorkspaceListPayload>(workspaceList.stdout, "workspaces list");
  const workspace = workspacePayload.workspaces?.find(entry => entry.root_dir === fixtureRoot);
  if (!workspace) {
    throw new Error(
      `resolved workspace was not present in workspaces list:\n${workspaceList.stdout}`
    );
  }

  const seededTaskRunId = await seedDryRun(
    paths.commandLogFile,
    paths.binaryPath,
    fixtureRoot,
    env,
    ["tasks", "run", PLAYWRIGHT_RUN_SEED_WORKFLOW_SLUG, "--dry-run", "--detach"]
  );
  const seededReviewRunId = await seedDryRun(
    paths.commandLogFile,
    paths.binaryPath,
    fixtureRoot,
    env,
    ["reviews", "fix", "daemon", "--round", "1", "--dry-run", "--detach"]
  );

  await waitForRunSnapshot(baseUrl, seededTaskRunId);
  await waitForRunSnapshot(baseUrl, seededReviewRunId);

  const environment: DaemonUIEnvironment = {
    baseUrl,
    workspaceId: workspace.id,
    workspaceName: workspace.name,
    fixtureRoot,
    homeDir,
    seededTaskRunId,
    seededReviewRunId,
  };
  await writeFile(paths.environmentFile, `${JSON.stringify(environment, null, 2)}\n`, "utf8");
}

async function assertBinaryExists(binaryPath: string): Promise<void> {
  try {
    const info = await stat(binaryPath);
    if (!info.isFile()) {
      throw new Error("not a file");
    }
  } catch (error) {
    throw new Error(`Playwright requires a built daemon binary at ${binaryPath}: ${String(error)}`);
  }
}

async function createWorkspaceFixture(repoRoot: string, fixtureRoot: string): Promise<void> {
  await mkdir(path.join(fixtureRoot, ".compozy", "tasks"), { recursive: true });
  for (const slug of PLAYWRIGHT_SOURCE_WORKFLOW_SLUGS) {
    await cp(
      path.join(repoRoot, ".compozy", "tasks", slug),
      path.join(fixtureRoot, ".compozy", "tasks", slug),
      { recursive: true }
    );
  }
  await createRunnableWorkflow(
    fixtureRoot,
    PLAYWRIGHT_RUN_SEED_WORKFLOW_SLUG,
    "Seed Runnable Task"
  );
  await createRunnableWorkflow(fixtureRoot, PLAYWRIGHT_START_WORKFLOW_SLUG, "Start Runnable Task");
  await createArchiveReadyWorkflow(fixtureRoot);
}

async function createRunnableWorkflow(
  fixtureRoot: string,
  slug: string,
  title: string
): Promise<void> {
  const workflowDir = path.join(fixtureRoot, ".compozy", "tasks", slug);
  await mkdir(workflowDir, { recursive: true });
  await writeFile(
    path.join(workflowDir, "task_001.md"),
    [
      "---",
      "status: pending",
      `title: ${title}`,
      "type: test",
      "complexity: low",
      "---",
      "",
      `# ${title}`,
      "",
      "Fixture workflow reserved for daemon-served run smoke coverage.",
      "",
    ].join("\n"),
    "utf8"
  );
}

async function createArchiveReadyWorkflow(fixtureRoot: string): Promise<void> {
  const workflowDir = path.join(fixtureRoot, ".compozy", "tasks", PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG);
  await mkdir(workflowDir, { recursive: true });
  await writeFile(
    path.join(workflowDir, "task_001.md"),
    [
      "---",
      "status: completed",
      "title: Archive Ready",
      "type: test",
      "complexity: low",
      "---",
      "",
      "# Archive Ready",
      "",
      "Fixture workflow reserved for daemon-served archive smoke coverage.",
      "",
    ].join("\n"),
    "utf8"
  );
}

async function seedDryRun(
  commandLogFile: string,
  binaryPath: string,
  cwd: string,
  env: NodeJS.ProcessEnv,
  args: string[]
): Promise<string> {
  const result = await runCLI(commandLogFile, binaryPath, args, { cwd, env });
  const combined = `${result.stdout}\n${result.stderr}`;
  const match = combined.match(/task run started:\s+([^\s]+)/i);
  if (!match?.[1]) {
    throw new Error(`unable to extract run id from command output:\n${combined}`);
  }
  return match[1];
}

async function waitForRunSnapshot(baseUrl: string, runId: string): Promise<void> {
  const deadline = Date.now() + 30_000;
  const url = `${baseUrl}/api/runs/${encodeURIComponent(runId)}/snapshot`;

  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
    } catch {
      // The daemon can still be binding the listener while the seed commands complete.
    }
    await new Promise(resolve => setTimeout(resolve, 250));
  }

  throw new Error(`timed out waiting for run snapshot: ${runId}`);
}

function daemonEnvironment(homeDir: string): NodeJS.ProcessEnv {
  return {
    ...process.env,
    HOME: homeDir,
  };
}

async function runCLI(
  commandLogFile: string,
  binaryPath: string,
  args: string[],
  options: {
    cwd: string;
    env: NodeJS.ProcessEnv;
  }
): Promise<CommandResult> {
  const result = spawnSync(binaryPath, args, {
    cwd: options.cwd,
    env: options.env,
    encoding: "utf8",
  });
  const stdout = result.stdout ?? "";
  const stderr = result.stderr ?? "";
  await appendFile(
    commandLogFile,
    [`$ ${[binaryPath, ...args].join(" ")}`, stdout.trimEnd(), stderr.trimEnd(), ""].join("\n"),
    "utf8"
  );

  if (result.status !== 0) {
    throw new Error(
      [
        `command failed: ${[binaryPath, ...args].join(" ")}`,
        `exit code: ${result.status ?? "unknown"}`,
        stdout ? `stdout:\n${stdout}` : "",
        stderr ? `stderr:\n${stderr}` : "",
      ]
        .filter(Boolean)
        .join("\n\n")
    );
  }

  return { stdout, stderr };
}

function parseJSON<T>(input: string, label: string): T {
  try {
    return JSON.parse(input) as T;
  } catch (error) {
    throw new Error(`failed to parse ${label} JSON: ${String(error)}\n${input}`);
  }
}
