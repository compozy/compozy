import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const STORYBOOK_HOST = "127.0.0.1";
const START_TIMEOUT_MS = 60_000;
const currentDir = path.dirname(fileURLToPath(import.meta.url));

export const webPackageRoot = path.resolve(currentDir, "../..");

export interface StorybookServer {
  baseURL: string;
  process: ChildProcessWithoutNullStreams;

  output: string[];
}

export function storyURL(baseURL: string, storyId: string): string {
  return `${baseURL}/iframe.html?id=${storyId}&viewMode=story`;
}

export function spawnStorybook(port: number, cwd = webPackageRoot): StorybookServer {
  const output: string[] = [];
  const child = spawn(
    "bunx",
    ["storybook", "dev", "--host", STORYBOOK_HOST, "--port", String(port), "--ci"],
    { cwd, env: process.env, stdio: "pipe" }
  );
  child.stdout.on("data", chunk => output.push(chunk.toString()));
  child.stderr.on("data", chunk => output.push(chunk.toString()));
  return { baseURL: `http://127.0.0.1:${port}`, process: child, output };
}

export async function waitForStorybook(server: StorybookServer): Promise<void> {
  await pollUntil(server, "Storybook to start", async () => {
    const response = await fetch(`${server.baseURL}/iframe.html`);
    return response.ok;
  });
}

export async function waitForStoryModule(
  server: StorybookServer,
  modulePath: string,
  expectedSymbol: string
): Promise<void> {
  await pollUntil(server, "the Storybook story module", async () => {
    const response = await fetch(`${server.baseURL}${modulePath}`);
    const body = await response.text();
    return response.ok && body.includes(expectedSymbol);
  });
}

export async function stopStorybook(server: StorybookServer): Promise<void> {
  const child = server.process;
  if (child.exitCode !== null) return;
  child.kill("SIGTERM");
  const exited = Promise.race([once(child, "exit"), delay(5_000).then(() => "timeout")]);
  if ((await exited) === "timeout" && child.exitCode === null) {
    child.kill("SIGKILL");
    await once(child, "exit");
  }
}

async function pollUntil(
  server: StorybookServer,
  what: string,
  probe: () => Promise<boolean>
): Promise<void> {
  const deadline = Date.now() + START_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (server.process.exitCode !== null) {
      throw new Error(
        `Storybook exited early with code ${server.process.exitCode}.\n${server.output.join("")}`
      );
    }
    try {
      if (await probe()) return;
    } catch {}
    await delay(500);
  }
  throw new Error(`Timed out waiting for ${what}.`);
}
