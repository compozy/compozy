import { execFile, spawn } from "node:child_process";
import { once } from "node:events";
import { readFile, writeFile } from "node:fs/promises";
import { createInterface } from "node:readline";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import { test as base } from "@playwright/test";

const execFileAsync = promisify(execFile);
const repo = fileURLToPath(new URL("../../../", import.meta.url));

type Scenario = "ssh-add" | "account-add" | "revoke-device";

interface Daemon {
  web_url: string;
  socket: string;
  home: string;
  operator_home: string;
  device_id: string;
}

interface Computer {
  id?: string;
  name: string;
  socket: string;
  container?: string;
}

interface RemoteComputersLab {
  scenario: Scenario;
  cloud_url: string;
  controller: Daemon;
  cli_controller?: Daemon;
  computers: [Computer, ...Computer[]];
  email: string;
  session_id?: string;
  session_title?: string;
}

export const test = base.extend<{ scenario: Scenario; lab: RemoteComputersLab }>({
  scenario: ["ssh-add", { option: true }],
  lab: [
    async ({ scenario }, use, testInfo) => {
      const binary = process.env.COMPOZY_E2E_REMOTETEST_BIN;
      if (!binary) {
        throw new Error("Run the Remote Computers scenarios through make test-e2e-web.");
      }
      const child = spawn(binary, ["-scenario", scenario, "-repo", repo], { cwd: repo });
      child.stdin.end();
      const closed = once(child, "close");
      const lines = createInterface({ input: child.stdout });
      let diagnostics = "";
      child.stderr.setEncoding("utf8").on("data", (chunk: string) => {
        diagnostics = (diagnostics + chunk).slice(-64_000);
      });
      let lab: RemoteComputersLab | undefined;
      let exitCode: number | null = null;
      try {
        const line = await Promise.race([
          once(lines, "line").then(([line]) => String(line)),
          closed.then(([code, signal]) => {
            throw new Error(
              `remotetest exited before readiness (${code ?? signal})\n${diagnostics}`
            );
          }),
        ]);
        lab = JSON.parse(line) as RemoteComputersLab;
        if (lab.scenario !== scenario || !lab.controller.web_url || !lab.computers.length) {
          throw new Error(`Invalid manifest for remotetest scenario ${scenario}`);
        }
        await use(lab);
      } finally {
        lines.close();
        const failed = testInfo.status !== testInfo.expectedStatus || !lab;
        const daemons = failed && lab ? [lab.controller, lab.cli_controller] : [];
        const logs = await Promise.all(
          daemons.map(
            daemon => daemon && readFile(`${daemon.home}/daemon.log`).catch(() => undefined)
          )
        );
        if (child.exitCode === null && child.signalCode === null) child.kill("SIGTERM");
        [exitCode] = await closed;
        for (const [index, log] of logs.entries()) {
          if (log) {
            const path = testInfo.outputPath(`remotetest-daemon-${index}.log`);
            await writeFile(path, log);
            await testInfo.attach(`remotetest-daemon-${index}`, {
              path,
              contentType: "text/plain",
            });
          }
        }
        if (diagnostics && (failed || exitCode !== 0)) {
          await testInfo.attach("remotetest", { body: diagnostics, contentType: "text/plain" });
        }
      }
      if (exitCode !== 0) {
        throw new Error(`remotetest teardown failed (${exitCode})\n${diagnostics}`);
      }
    },
    { timeout: 540_000 },
  ],
  baseURL: async ({ lab }, use) => {
    await use(lab.controller.web_url);
  },
});

export { expect } from "@playwright/test";

interface ComputerStatus {
  status?: string;
  code?: string;
  diagnosis?: { code: string };
}

export async function cliComputerStatus(
  lab: RemoteComputersLab,
  name: string
): Promise<ComputerStatus> {
  const binary = process.env.COMPOZY_TEST_DAEMON_BIN;
  const device = lab.cli_controller;
  if (!binary || !device) throw new Error("The scenario requires the second CLI controller.");
  const failureOutput = (error: unknown): string => {
    if (!(error instanceof Error)) throw error;
    const command = error as Error & { stdout?: string; stderr?: string };
    const output = command.stdout?.trim() || command.stderr?.trim();
    if (!output) throw error;
    return output;
  };
  const output = await execFileAsync(binary, ["computer", "status", name, "--json"], {
    cwd: device.operator_home,
    env: {
      ...process.env,
      HOME: device.operator_home,
      COMPOZY_HOME: device.home,
      COMPOZY_CLOUD_URL: lab.cloud_url,
      COMPOZY_SESSION_ID: "",
      COMPOZY_AGENT: "",
      COMPOZY_PROFILE: "",
      COMPOZY_WORKSPACE: "",
    },
    timeout: 1_000,
  }).then(result => result.stdout, failureOutput);
  return JSON.parse(output) as ComputerStatus;
}
