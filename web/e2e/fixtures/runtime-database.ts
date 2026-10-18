import { execFile } from "node:child_process";
import { constants, rmSync } from "node:fs";
import { copyFile, mkdtemp, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const templates = new Map<string, Promise<string>>();
const templateDirs = new Set<string>();
const databaseName = "compozy.db";

// Like the browser assets and API fixtures, this seed belongs to the current
// checkout. COMPOZY_TEST_DAEMON_BIN must be built from that same checkout.
// Only closed schema/default data is shared; each daemon opens its own copy.
export async function seedRuntimeDatabase(repoRoot: string, homeDir: string): Promise<void> {
  let template = templates.get(repoRoot);
  if (template === undefined) {
    template = createTemplate(repoRoot).catch(error => {
      templates.delete(repoRoot);
      throw error;
    });
    templates.set(repoRoot, template);
  }
  const sourcePath = await template;
  for (const suffix of ["", "-wal", "-shm"]) {
    try {
      await copyFile(
        sourcePath + suffix,
        path.join(homeDir, databaseName + suffix),
        constants.COPYFILE_EXCL
      );
    } catch (error) {
      if (suffix !== "" && (error as NodeJS.ErrnoException).code === "ENOENT") continue;
      throw error;
    }
  }
}

async function createTemplate(repoRoot: string): Promise<string> {
  const dir = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-database-"));
  templateDirs.add(dir);
  const output = path.join(dir, databaseName);
  try {
    const override = process.env.COMPOZY_TEST_DATABASE_SEEDER_BIN?.trim();
    const binaryPath = override
      ? path.resolve(repoRoot, override)
      : path.join(dir, process.platform === "win32" ? "database-seeder.exe" : "database-seeder");
    const execOptions = { cwd: repoRoot, maxBuffer: 20 * 1024 * 1024 };
    // Mage builds once before starting Playwright workers. Direct fixture runs
    // build once per cached template, outside subsequent per-test database copies.
    if (!override) {
      await execFileAsync(
        "go",
        ["build", "-p", "2", "-o", binaryPath, "./web/e2e/fixtures/runtime-database-seeder"],
        execOptions
      );
    }
    await execFileAsync(binaryPath, ["--output", output], execOptions);
    return output;
  } catch (error) {
    await rm(dir, { force: true, recursive: true });
    templateDirs.delete(dir);
    throw error;
  }
}

process.once("exit", () => {
  for (const dir of templateDirs) {
    try {
      rmSync(dir, { force: true, recursive: true });
    } catch (error) {
      process.stderr.write(`failed to remove browser database template: ${String(error)}\n`);
    }
  }
});
