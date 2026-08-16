import { execFile } from "node:child_process";
import { access, cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, dirname, join, resolve } from "node:path";

import {
  _electron as electron,
  test as base,
  type ElectronApplication,
  type Page,
} from "@playwright/test";

interface LaunchContext {
  readonly bundleRuntimePath: string;
  readonly environment: Readonly<Record<string, string>>;
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
  cli(arguments_: readonly string[]): Promise<{ readonly stdout: string; readonly stderr: string }>;
  closeShell(): Promise<void>;
  product(): Promise<Page>;
  spawnSecondary(arguments_: readonly string[]): Promise<void>;
}

export interface PackagedCopy {
  readonly bundleRuntimePath: string;
  readonly executablePath: string;
  readonly mainPath: string;
  readonly resourcesPath: string;
  readonly root: string;
}

interface DesktopFixtures {
  copyPackagedExecutable(): Promise<PackagedCopy>;
  launchDesktop(options?: LaunchOptions): Promise<DesktopInstance>;
}

function packagedExecutable(): string {
  const configured = process.env.COMPOZY_DESKTOP_EXECUTABLE?.trim();
  if (configured) return resolve(configured);
  if (process.platform === "darwin") {
    const architecture = process.arch === "arm64" ? "arm64" : "x64";
    return resolve(`dist/mac-${architecture}/CompozyOS.app/Contents/MacOS/CompozyOS`);
  }
  if (process.platform === "linux") return resolve("dist/linux-unpacked/compozyos");
  return resolve("dist/win-unpacked/CompozyOS.exe");
}

function packagePaths(executablePath: string): Omit<PackagedCopy, "root"> {
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
    mainPath: join(resourcesPath, "app.asar"),
  };
}

function compactEnvironment(environment: NodeJS.ProcessEnv): Record<string, string> {
  return Object.fromEntries(
    Object.entries(environment).filter((entry): entry is [string, string] => entry[1] !== undefined)
  );
}

async function command(
  executable: string,
  arguments_: readonly string[],
  environment: NodeJS.ProcessEnv,
  timeout = 30_000
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
  try {
    await command(runtime, ["daemon", "stop"], environment);
    return;
  } catch (stopError) {
    let metadata: unknown;
    try {
      metadata = JSON.parse(await readFile(join(home, "daemon.json"), "utf8"));
    } catch (metadataError) {
      if (
        metadataError instanceof Error &&
        "code" in metadataError &&
        metadataError.code === "ENOENT"
      ) {
        return;
      }
      throw new AggregateError([stopError, metadataError], "Could not inspect the E2E runtime.");
    }
    const pid =
      metadata && typeof metadata === "object" && typeof Reflect.get(metadata, "pid") === "number"
        ? Reflect.get(metadata, "pid")
        : 0;
    if (!Number.isSafeInteger(pid) || pid < 1) throw stopError;
    try {
      process.kill(pid, "SIGTERM");
    } catch (killError) {
      if (killError instanceof Error && "code" in killError && killError.code === "ESRCH") return;
      throw new AggregateError([stopError, killError], "Could not stop the E2E runtime.");
    }
  }
}

async function terminateRestartedDesktop(home: string): Promise<void> {
  let record: unknown;
  try {
    record = JSON.parse(await readFile(join(home, "app.json"), "utf8"));
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return;
    throw error;
  }
  const pid =
    record && typeof record === "object" && typeof Reflect.get(record, "pid") === "number"
      ? Reflect.get(record, "pid")
      : 0;
  if (!Number.isSafeInteger(pid) || pid < 1) return;
  try {
    process.kill(pid, 0);
    process.kill(pid, "SIGTERM");
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ESRCH") return;
    throw error;
  }
}

export const test = base.extend<DesktopFixtures>({
  launchDesktop: async (_fixtures, use) => {
    const instances: DesktopInstance[] = [];
    const homes = new Set<string>();
    await use(async (options = {}) => {
      const home = options.home ?? (await mkdtemp(join(tmpdir(), "compozy-electron-e2e-")));
      if (options.home && !homes.has(home)) {
        throw new Error("A desktop E2E home can only be reused after this fixture created it.");
      }
      homes.add(home);
      const executablePath = options.executablePath ?? packagedExecutable();
      await access(executablePath);
      const paths = packagePaths(executablePath);
      const environment = compactEnvironment({ ...process.env, ...options.environment });
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
      const boot = await app.firstWindow();
      let closed = false;
      const instance: DesktopInstance = {
        app,
        boot,
        bundleRuntimePath: paths.bundleRuntimePath,
        environment,
        executablePath,
        home,
        async cli(arguments_) {
          return await command(
            join(home, "bin", process.platform === "win32" ? "compozy.exe" : "compozy"),
            arguments_,
            environment
          );
        },
        async product() {
          const existing = app
            .windows()
            .find(
              window => window.url().startsWith("http://") || window.url().startsWith("https://")
            );
          if (existing) return existing;
          return await app.waitForEvent("window", {
            predicate: window =>
              window.url().startsWith("http://") || window.url().startsWith("https://"),
          });
        },
        async closeShell() {
          if (closed) return;
          closed = true;
          if (app.process().exitCode !== null) return;
          await app.close();
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
      try {
        await terminateRestartedDesktop(home);
        await terminateRuntime(home, { ...process.env, COMPOZY_HOME: home });
        await rm(home, { recursive: true, force: true });
      } catch (error) {
        cleanupError = cleanupError ? new AggregateError([cleanupError, error]) : error;
      }
    }
    if (cleanupError) throw cleanupError;
  },
  copyPackagedExecutable: async (_fixtures, use) => {
    const roots: string[] = [];
    await use(async () => {
      const source = packagedExecutable();
      const root = await mkdtemp(join(tmpdir(), "compozy-electron-package-"));
      roots.push(root);
      if (process.platform === "darwin") {
        const sourceApp = resolve(source, "..", "..", "..");
        const targetApp = join(root, "CompozyOS.app");
        await cp(sourceApp, targetApp, { recursive: true });
        return { ...packagePaths(join(targetApp, "Contents", "MacOS", "CompozyOS")), root };
      }
      const sourceRoot = dirname(source);
      const targetRoot = join(root, basename(sourceRoot));
      await cp(sourceRoot, targetRoot, { recursive: true });
      return { ...packagePaths(join(targetRoot, basename(source))), root };
    });
    for (const root of roots) await rm(root, { recursive: true, force: true });
  },
});

export { expect } from "@playwright/test";
