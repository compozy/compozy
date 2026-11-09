// The seeded web app behind export:product-shots: an isolated daemon on a short /tmp home, seeded
// by scripts/demo-seed and serving the web/ build on its own port. Never the operator's daemon:
// COMPOZY_* is stripped from the environment and COMPOZY_HOME points at the throwaway home.
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { join, resolve } from "node:path";

export interface SeededApp {
  base: string;
  stop: () => void;
}

/** The parent environment minus every COMPOZY_* variable (a managed session leaks its own). */
function cleanEnv(extra: Record<string, string>): NodeJS.ProcessEnv {
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([key]) => !key.startsWith("COMPOZY_"))
  );
  return { ...env, ...extra };
}

function run(cmd: string, args: string[], cwd: string, env: NodeJS.ProcessEnv, what: string) {
  const out = spawnSync(cmd, args, { cwd, env, stdio: "inherit" });
  if (out.status !== 0) throw new Error(`${what} failed (exit ${out.status})`);
}

function freePort(): Promise<number> {
  return new Promise((ok, fail) => {
    const server = createServer();
    server.once("error", fail);
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address();
      const port = typeof addr === "object" && addr ? addr.port : 0;
      server.close(() => ok(port));
    });
  });
}

async function waitForApi(base: string, timeoutMs: number) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(`${base}/api/workspaces`)).ok) return;
    } catch {
      // daemon still starting
    }
    await new Promise(done => setTimeout(done, 500));
  }
  throw new Error(`seeded daemon did not answer on ${base} within ${timeoutMs / 1000}s`);
}

/**
 * Builds the CLI and web/, seeds a fresh home and starts its daemon. `webDist` reuses an existing
 * web/ build. The returned `stop` stops the daemon and deletes the home.
 */
export async function startSeededApp(repo: string, webDist?: string): Promise<SeededApp> {
  const root = mkdtempSync("/tmp/bs-");
  const home = join(root, "h");
  const bin = join(root, "compozy");
  const dist = webDist ? resolve(webDist) : join(repo, "web/dist");
  const port = await freePort();
  const env = cleanEnv({ COMPOZY_HOME: home, COMPOZY_WEB_DIST_DIR: dist });
  let started = false;
  const stop = () => {
    if (started) spawnSync(bin, ["daemon", "stop"], { cwd: repo, env, stdio: "ignore" });
    rmSync(root, { recursive: true, force: true });
  };
  try {
    console.log(`seeded app: home ${home}, port ${port}`);
    run("go", ["build", "-o", bin, "./cmd/compozy"], repo, env, "go build ./cmd/compozy");
    if (!webDist) run("bun", ["run", "build"], join(repo, "web"), env, "web/ build");
    run("go", ["run", "./scripts/demo-seed", "--home", home, "--replace"], repo, env, "demo-seed");
    run(bin, ["config", "set", "http.port", String(port), "--scope", "user"], repo, env, "config");
    run(bin, ["daemon", "start"], repo, env, "daemon start");
    started = true;
    const base = `http://127.0.0.1:${port}`;
    await waitForApi(base, 60_000);
    return { base, stop };
  } catch (error) {
    stop();
    throw error;
  }
}
