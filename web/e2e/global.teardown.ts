import { appendFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";

import type { FullConfig } from "@playwright/test";

import { loadDaemonUIEnvironment, resolvePlaywrightPaths } from "./support/daemon-fixture";

export default async function globalTeardown(_config: FullConfig): Promise<void> {
  const paths = resolvePlaywrightPaths();

  try {
    const environment = await loadDaemonUIEnvironment();
    const result = spawnSync(paths.binaryPath, ["daemon", "stop", "--force", "--format", "json"], {
      cwd: environment.fixtureRoot,
      env: {
        ...process.env,
        HOME: environment.homeDir,
      },
      encoding: "utf8",
    });
    await appendFile(
      paths.commandLogFile,
      [
        `$ ${[paths.binaryPath, "daemon", "stop", "--force", "--format", "json"].join(" ")}`,
        (result.stdout ?? "").trimEnd(),
        (result.stderr ?? "").trimEnd(),
        "",
      ].join("\n"),
      "utf8"
    );
  } catch {
    // Keep teardown best-effort so Playwright can still emit its report when setup fails midway.
  }
}
