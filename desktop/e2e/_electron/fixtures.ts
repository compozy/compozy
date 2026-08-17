import { execFile, type ChildProcess } from "node:child_process";
import { access, cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  _electron as electron,
  test as base,
  type ElectronApplication,
  type Page,
} from "@playwright/test";

import { readDiagnosticFile } from "./helpers";

const repositoryRoot = resolve(fileURLToPath(new URL(".", import.meta.url)), "../../..");

interface LaunchContext {
  readonly bundleRuntimePath: string;
  readonly environment: Record<string, string>;
  readonly executablePath: string;
  readonly home: string;
  readonly resourcesPath: string;
}

interface LaunchOptions {
  readonly argv?: readonly string[];
  readonly environment?: NodeJS.ProcessEnv;
  readonly executablePath?: string;
  readonly home?: string;
  readonly prepare?: (context: LaunchContext) => Promise<void>;
}

export interface DesktopInstance {
  readonly app: ElectronApplication;
  readonly boot: Page;
  readonly bundleRuntimePath: string;
  readonly environment: Readonly<Record<string, string>>;
  readonly executablePath: string;
  readonly home: string;
  readonly shellProcess: ChildProcess;
  cli(arguments_: readonly string[]): Promise<{ readonly stdout: string; readonly stderr: string }>;
  closeShell(): Promise<void>;
  product(): Promise<Page>;
  spawnSecondary(arguments_: readonly string[]): Promise<void>;
}

export interface PackagedCopy {
  readonly executablePath: string;
  readonly resourcesPath: string;
}

interface DesktopFixtures {
  copyPackagedExecutable(sourceExecutable?: string): Promise<PackagedCopy>;
  launchDesktop(options?: LaunchOptions): Promise<DesktopInstance>;
}

function packagedExecutable(): string {
  const configured = process.env.COMPOZY_DESKTOP_EXECUTABLE?.trim();
  if (configured) return resolve(configured);
  if (process.platform === "darwin") {
    const architecture = process.arch === "arm64" ? "arm64" : "x64";
    return resolve(
      `.artifacts/e2e-baseline/mac-${architecture}/CompozyOS.app/Contents/MacOS/CompozyOS`
    );
  }
  if (process.platform === "linux") {
    return resolve(".artifacts/e2e-baseline/linux-unpacked/compozyos");
  }
  return resolve(".artifacts/e2e-baseline/win-unpacked/CompozyOS.exe");
}

export function productionPackagedExecutable(): string {
  if (process.platform === "darwin") {
    const architecture = process.arch === "arm64" ? "arm64" : "x64";
    return resolve(`.artifacts/builder/mac-${architecture}/CompozyOS.app/Contents/MacOS/CompozyOS`);
  }
  if (process.platform === "linux") {
    return resolve(".artifacts/builder/linux-unpacked/compozyos");
  }
  return resolve(".artifacts/builder/win-unpacked/CompozyOS.exe");
}

interface PackagePaths extends PackagedCopy {
  readonly bundleRuntimePath: string;
}

function packagePaths(executablePath: string): PackagePaths {
  const resourcesPath =
    process.platform === "darwin"
      ? resolve(dirname(executablePath), "..", "Resources")
      : join(dirname(executablePath), "resources");
  return {
    executablePath,
    resourcesPath,
    bundleRuntimePath: join(
      resourcesPath,
      "runtime",
      process.platform === "win32" ? "compozy.exe" : "compozy"
    ),
  };
}

function compactEnvironment(environment: NodeJS.ProcessEnv): Record<string, string> {
  return Object.fromEntries(
    Object.entries(environment).filter((entry): entry is [string, string] => entry[1] !== undefined)
  );
}

export async function availableLoopbackPort(): Promise<number> {
  const server = createServer();
  await new Promise<void>((resolveListen, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", reject);
      resolveListen();
    });
  });
  const address = server.address();
  if (!address || typeof address === "string") {
    await new Promise<void>((resolveClose, reject) =>
      server.close(error => (error ? reject(error) : resolveClose()))
    );
    throw new Error("The desktop E2E port reservation did not expose a TCP port.");
  }
  await new Promise<void>((resolveClose, reject) =>
    server.close(error => (error ? reject(error) : resolveClose()))
  );
  return address.port;
}

async function command(
  executable: string,
  arguments_: readonly string[],
  environment: NodeJS.ProcessEnv,
  timeout = 5 * 60_000
): Promise<{ readonly stdout: string; readonly stderr: string }> {
  return await new Promise((resolveCommand, reject) => {
    execFile(
      executable,
      [...arguments_],
      { env: environment, encoding: "utf8", timeout },
      (error, stdout, stderr) => {
        if (error) {
          reject(
            new Error(stderr.trim() || `Command ${basename(executable)} failed.`, { cause: error })
          );
          return;
        }
        resolveCommand({ stdout, stderr });
      }
    );
  });
}

async function registerLinuxPackage(home: string, executablePath: string): Promise<string> {
  const operatorHome = join(home, "operator");
  const applications = join(operatorHome, ".local", "share", "applications");
  await mkdir(applications, { recursive: true, mode: 0o700 });
  await writeFile(
    join(applications, "com.compozy.os.desktop"),
    [
      "[Desktop Entry]",
      "Name=CompozyOS",
      `Exec=${executablePath}`,
      "X-Compozy-Version=0.3.0",
      "Type=Application",
      "",
    ].join("\n"),
    { encoding: "utf8", mode: 0o600 }
  );
  return operatorHome;
}

async function terminateRuntime(home: string, environment: NodeJS.ProcessEnv): Promise<void> {
  const runtime = join(home, "bin", process.platform === "win32" ? "compozy.exe" : "compozy");
  try {
    await access(runtime);
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return;
    throw error;
  }
  let pid: number | null = null;
  try {
    const record = JSON.parse(await readFile(join(home, "daemon.json"), "utf8")) as Record<
      string,
      unknown
    >;
    if (Number.isSafeInteger(record.pid) && Number(record.pid) > 0) pid = Number(record.pid);
  } catch (error) {
    if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) throw error;
  }
  await command(runtime, ["daemon", "stop"], environment);
  if (pid !== null) await waitForProcessExit(pid);
  await waitForPathRemoval(join(home, "daemon.sock"));
}

async function waitForProcessExit(pid: number): Promise<void> {
  const deadline = Date.now() + 5_000;
  while (Date.now() < deadline) {
    try {
      process.kill(pid, 0);
    } catch (error) {
      if (error instanceof Error && "code" in error && error.code === "ESRCH") return;
      throw error;
    }
    await new Promise(resolveDelay => setTimeout(resolveDelay, 50));
  }
  throw new Error(`Desktop E2E process ${pid} did not stop within 5 seconds.`);
}

async function waitForPathRemoval(path: string): Promise<void> {
  const deadline = Date.now() + 5_000;
  while (Date.now() < deadline) {
    try {
      await access(path);
    } catch (error) {
      if (error instanceof Error && "code" in error && error.code === "ENOENT") return;
      throw error;
    }
    await new Promise(resolveDelay => setTimeout(resolveDelay, 50));
  }
  throw new Error(`Desktop E2E path ${basename(path)} was not removed within 5 seconds.`);
}

async function terminateRestartedDesktop(home: string): Promise<void> {
  let record: unknown;
  try {
    record = JSON.parse(await readFile(join(home, "app.json"), "utf8"));
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return;
    throw error;
  }
  const pid = record && typeof record === "object" ? Reflect.get(record, "pid") : undefined;
  if (!Number.isSafeInteger(pid) || Number(pid) < 1) return;
  try {
    process.kill(Number(pid), "SIGTERM");
    await waitForProcessExit(Number(pid));
    await waitForPathRemoval(join(home, "app.sock"));
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ESRCH") return;
    throw error;
  }
}

export const test = base.extend<DesktopFixtures>({
  launchDesktop: async ({ playwright: _playwright }, use) => {
    const instances: DesktopInstance[] = [];
    const homes = new Set<string>();
    await use(async (options = {}) => {
      const home = options.home ?? (await mkdtemp(join(tmpdir(), "compozy-electron-e2e-")));
      if (options.home && !homes.has(home)) {
        throw new Error("A desktop E2E home can only be reused after this fixture created it.");
      }
      if (!homes.has(home)) {
        const port = await availableLoopbackPort();
        await writeFile(
          join(home, "config.toml"),
          `[app]\nupdate_check = false\n\n[http]\nhost = "127.0.0.1"\nport = ${port}\n`,
          { flag: "wx", mode: 0o600 }
        );
        homes.add(home);
      }
      const executablePath = options.executablePath ?? packagedExecutable();
      await access(executablePath);
      const paths = packagePaths(executablePath);
      const environment = compactEnvironment({ ...process.env, ...options.environment });
      environment.COMPOZY_DESKTOP_E2E = "1";
      const productReadyPath = join(home, "e2e-product-ready");
      await rm(productReadyPath, { force: true });
      environment.COMPOZY_DESKTOP_E2E_READY_FILE = productReadyPath;
      environment.COMPOZY_WEB_DIST_DIR = join(repositoryRoot, "web", "dist");
      if (process.platform === "linux") {
        environment.HOME = await registerLinuxPackage(home, executablePath);
      }
      environment.COMPOZY_HOME = home;
      const context: LaunchContext = { ...paths, environment, home };
      await options.prepare?.(context);
      const app = await electron.launch({
        executablePath,
        args: [...(options.argv ?? [])],
        env: environment,
      });
      const launchedProcess = app.process();
      const processOutput: string[] = [];
      for (const stream of [launchedProcess.stdout, launchedProcess.stderr]) {
        stream?.on("data", chunk => processOutput.push(String(chunk)));
      }
      let boot: Page;
      try {
        await writeFile(productReadyPath, "ready\n", { flag: "wx", mode: 0o600 });
        boot = await app.firstWindow();
      } catch (error) {
        try {
          await app.close();
        } catch (closeError) {
          const pid = launchedProcess.pid;
          let terminationError: unknown;
          try {
            if (!pid) throw new Error("The failed desktop launch has no process id.");
            launchedProcess.kill("SIGTERM");
            await waitForProcessExit(pid);
          } catch (killError) {
            terminationError = killError;
          }
          throw new AggregateError(
            [error, closeError, ...(terminationError ? [terminationError] : [])],
            "Desktop E2E setup failed and the application could not close cleanly."
          );
        }
        throw error;
      }
      let closed = false;
      app.on("close", () => {
        closed = true;
      });
      const instance: DesktopInstance = {
        app,
        boot,
        bundleRuntimePath: paths.bundleRuntimePath,
        environment,
        executablePath,
        home,
        shellProcess: launchedProcess,
        async cli(arguments_) {
          return await command(
            join(home, "bin", process.platform === "win32" ? "compozy.exe" : "compozy"),
            arguments_,
            environment
          );
        },
        async product() {
          try {
            const deadline = Date.now() + 20_000;
            while (Date.now() < deadline) {
              for (const candidate of app.windows().toReversed()) {
                if (
                  !candidate.url().startsWith("http://") &&
                  !candidate.url().startsWith("https://")
                ) {
                  continue;
                }
                try {
                  await candidate.waitForLoadState("load");
                  await candidate.title();
                  return candidate;
                } catch {
                  // A crashed renderer can remain in Playwright's page list until Electron destroys it.
                }
              }
              await new Promise(resolveDelay => setTimeout(resolveDelay, 50));
            }
            throw new Error("The desktop product page was not observable within 20 seconds.");
          } catch (error) {
            const [desktopLog, bootstrapLog] = await Promise.all([
              readDiagnosticFile(join(home, "logs", "desktop.log")),
              readDiagnosticFile(join(home, "logs", "desktop-bootstrap.jsonl")),
            ]);
            throw new Error(
              `The desktop product window did not open (exit=${launchedProcess.exitCode ?? "running"}, signal=${launchedProcess.signalCode ?? "none"}).\nProcess output:\n${processOutput.join("").trim() || "(empty)"}\nDesktop log:\n${desktopLog}\nBootstrap log:\n${bootstrapLog}`,
              { cause: error }
            );
          }
        },
        async closeShell() {
          if (closed) return;
          const closeResult = app
            .close()
            .then(() => ({ error: null }))
            .catch((error: unknown) => ({ error }));
          const result = await Promise.race([
            closeResult,
            new Promise<null>(resolveTimeout => setTimeout(() => resolveTimeout(null), 5_000)),
          ]);
          if (result?.error) throw result.error;
          if (result !== null || launchedProcess.exitCode !== null) {
            closed = true;
            return;
          }
          const pid = launchedProcess.pid;
          if (!pid) throw new Error("The desktop E2E app has no process id.");
          launchedProcess.kill("SIGTERM");
          await waitForProcessExit(pid);
          closed = true;
        },
        async spawnSecondary(arguments_) {
          await command(executablePath, arguments_, environment, 15_000);
        },
      };
      instances.push(instance);
      return instance;
    });
    let cleanupError: unknown;
    for (const instance of instances.reverse()) {
      try {
        await instance.closeShell();
      } catch (error) {
        cleanupError = cleanupError ? new AggregateError([cleanupError, error]) : error;
      }
    }
    for (const home of homes) {
      const environment = { ...process.env, COMPOZY_HOME: home };
      for (const step of [
        async () => await terminateRestartedDesktop(home),
        async () => await terminateRuntime(home, environment),
        async () => await rm(home, { recursive: true, force: true }),
      ]) {
        try {
          await step();
        } catch (error) {
          cleanupError = cleanupError ? new AggregateError([cleanupError, error]) : error;
        }
      }
    }
    if (cleanupError) throw cleanupError;
  },
  copyPackagedExecutable: async ({ playwright: _playwright }, use) => {
    const roots: string[] = [];
    await use(async sourceExecutable => {
      const source = sourceExecutable ? resolve(sourceExecutable) : packagedExecutable();
      const root = await mkdtemp(join(tmpdir(), "compozy-electron-package-"));
      roots.push(root);
      if (process.platform === "darwin") {
        const sourceApp = resolve(source, "..", "..", "..");
        const targetApp = join(root, "CompozyOS.app");
        await cp(sourceApp, targetApp, { recursive: true, verbatimSymlinks: true });
        const paths = packagePaths(join(targetApp, "Contents", "MacOS", "CompozyOS"));
        return { executablePath: paths.executablePath, resourcesPath: paths.resourcesPath };
      }
      const sourceRoot = dirname(source);
      const targetRoot = join(root, basename(sourceRoot));
      await cp(sourceRoot, targetRoot, { recursive: true, verbatimSymlinks: true });
      const paths = packagePaths(join(targetRoot, basename(source)));
      return { executablePath: paths.executablePath, resourcesPath: paths.resourcesPath };
    });
    for (const root of roots) await rm(root, { recursive: true, force: true });
  },
});

export { expect } from "@playwright/test";
