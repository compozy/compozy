import { spawn } from "node:child_process";
import { resolve } from "node:path";
import { createServer } from "vite";

import { galleryOrigin, galleryRoot, repoRoot } from "../node/paths.ts";

export async function startGallery() {
  const server = await createServer({ configFile: resolve(galleryRoot, "vite.config.ts") });
  await server.listen();
  return server;
}

/** A gallery answers its comments API (`{ counts }`); any other server on the port does not. */
async function running() {
  try {
    const response = await fetch(`${galleryOrigin}/__comments`, {
      signal: AbortSignal.timeout(1500),
    });
    if (!response.ok) return false;
    const body = (await response.json()) as unknown;
    return typeof body === "object" && body !== null && "counts" in body;
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
  // Set by the exit listener, so a signal exit (exitCode stays null) counts too.
  let exited = false;
  const exit = new Promise<void>(done =>
    child.once("exit", () => {
      exited = true;
      done();
    })
  );
  const stop = async () => {
    if (exited) return;
    child.kill("SIGTERM");
    await exit;
  };
  const deadline = Date.now() + 120_000;
  while (!(await running())) {
    if (exited) throw new Error("proto dev exited before it was ready");
    if (Date.now() > deadline) {
      await stop();
      throw new Error(`proto dev did not answer on ${galleryOrigin}`);
    }
    await new Promise(done => setTimeout(done, 300));
  }
  return { origin: galleryOrigin, stop };
}
