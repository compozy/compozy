import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

export interface PlaywrightPaths {
  supportDir: string;
  webRoot: string;
  repoRoot: string;
  sharedTmpDir: string;
  environmentFile: string;
  commandLogFile: string;
  binaryPath: string;
}

export interface DaemonUIEnvironment {
  baseUrl: string;
  workspaceId: string;
  workspaceName: string;
  fixtureRoot: string;
  homeDir: string;
  seededTaskRunId: string;
  seededReviewRunId: string;
}

export const PLAYWRIGHT_SOURCE_WORKFLOW_SLUGS = ["daemon", "daemon-web-ui"] as const;
export const PLAYWRIGHT_RUN_SEED_WORKFLOW_SLUG = "playwright-run-seed";
export const PLAYWRIGHT_START_WORKFLOW_SLUG = "playwright-start-run";
export const PLAYWRIGHT_WORKFLOW_SLUGS = [
  ...PLAYWRIGHT_SOURCE_WORKFLOW_SLUGS,
  PLAYWRIGHT_RUN_SEED_WORKFLOW_SLUG,
  PLAYWRIGHT_START_WORKFLOW_SLUG,
] as const;
export const PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG = "archive-ready";

export function resolvePlaywrightPaths(): PlaywrightPaths {
  const supportDir = path.dirname(fileURLToPath(import.meta.url));
  const webRoot = path.resolve(supportDir, "..", "..");
  const repoRoot = path.resolve(webRoot, "..");
  const sharedTmpDir = path.join(repoRoot, ".tmp", "playwright");

  return {
    supportDir,
    webRoot,
    repoRoot,
    sharedTmpDir,
    environmentFile:
      process.env.COMPOZY_PLAYWRIGHT_ENV_FILE ?? path.join(sharedTmpDir, "daemon-ui-env.json"),
    commandLogFile: path.join(sharedTmpDir, "daemon-ui-commands.log"),
    binaryPath: path.join(repoRoot, "bin", "compozy"),
  };
}

export async function loadDaemonUIEnvironment(
  filePath = resolvePlaywrightPaths().environmentFile
): Promise<DaemonUIEnvironment> {
  const raw = await readFile(filePath, "utf8");
  return JSON.parse(raw) as DaemonUIEnvironment;
}
