import { spawn } from "node:child_process";
import { resolve } from "node:path";
import { createServer } from "vite";

import { galleryOrigin, galleryRoot, repoRoot } from "../node/paths.ts";

export async function startGallery() {
  const server = await createServer({ configFile: resolve(galleryRoot, "vite.config.ts") });
  await server.listen();
  return server;
}

async function running() {
  try {
    const response = await fetch(galleryOrigin, { signal: AbortSignal.timeout(1500) });
    return response.ok;
  } catch {
    return false;
  }
}

/**
 * The gallery origin, reusing a `proto dev` already listening on port 4400 or starting one in a
 * child process for the caller (a browser driven from the server's own process stalls its
 * module requests); `stop` ends only a server this call started.
 */
export async function ensureGallery(): Promise<{ origin: string; stop: () => Promise<void> }> {
  if (await running()) return { origin: galleryOrigin, stop: async () => undefined };
  const child = spawn(process.execPath, [resolve(galleryRoot, "cli/main.ts"), "dev"], {
    cwd: repoRoot,
    stdio: "ignore",
  });
  const stop = async () => {
    if (child.exitCode !== null) return;
    const exited = new Promise(done => child.once("exit", done));
    child.kill("SIGTERM");
    await exited;
  };
  const deadline = Date.now() + 120_000;
  while (!(await running())) {
    if (child.exitCode !== null) throw new Error("proto dev exited before it was ready");
    if (Date.now() > deadline) {
      await stop();
      throw new Error(`proto dev did not answer on ${galleryOrigin}`);
    }
    await new Promise(done => setTimeout(done, 300));
  }
  return { origin: galleryOrigin, stop };
}
