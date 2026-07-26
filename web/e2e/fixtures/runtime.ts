import { mkdir, mkdtemp, readFile, symlink, writeFile, copyFile } from "node:fs/promises";
import { createWriteStream, existsSync } from "node:fs";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import type { Server } from "node:http";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

import { ArtifactCollector } from "./artifacts";
import { closeMarketplaceCatalogServer, startMarketplaceCatalogServer } from "./marketplace-server";
import {
  applyBrowserRuntimeSeed,
  seedBrowserRuntimeHome,
  type BrowserRuntimeSeed,
  type BrowserRuntimeSeedResult,
  type WorkspacePayload,
} from "./runtime-seed";
import {
  assertDaemonServedHTML,
  buildLaunchRuntimeEnv,
  buildResolveWorkspaceRequest,
  prependPath,
  requiresHTTPAPIReadinessProbe,
  renderRuntimeConfig,
  resolveRuntimeMode,
  runtimeURL,
  type RuntimeMode,
} from "./runtime-helpers";
import { stopBrowserDaemonProcess, stopSpawnedDaemonProcess } from "./runtime-process";
import {
  closeSkillMarketplaceServer,
  startSkillMarketplaceServer,
  type SkillMarketplaceTestServer,
} from "./skill-marketplace-server";

export {
  closeSkillMarketplaceServer,
  startSkillMarketplaceServer,
  type SkillMarketplaceTestServer,
} from "./skill-marketplace-server";

const DEFAULT_HOST = "127.0.0.1";
const DEFAULT_READY_TIMEOUT_MS = 30_000;
const DEFAULT_READY_POLL_MS = 200;
const DAEMON_BINARY_ENV_VAR = "AGH_TEST_DAEMON_BIN";
const RUNTIME_CONTROL_ENV_VARS = [
  "AGH_E2E_BASE_URL",
  "AGH_TEST_ACPMOCK_DRIVER_BIN",
  DAEMON_BINARY_ENV_VAR,
  "AGH_WEB_DIST_DIR",
] as const;

let daemonBinaryPromise: Promise<string> | undefined;

export interface RuntimePaths {
  homeDir: string;
  configFile: string;
  daemonSocket: string;
  daemonLog: string;
  cliShim: string;
}

export interface BrowserRuntimeOptions {
  artifactRootDir: string;
  env?: NodeJS.ProcessEnv;
  extensionsAllowUnverified?: boolean;
  host?: string;
  modelsDevEnabled?: boolean;
  networkEnabled?: boolean;
  readyTimeoutMs?: number;
  seed?: BrowserRuntimeSeed;
  toolsExternalDefault?: "disabled" | "ask" | "enabled";
}

export interface BrowserRuntime {
  readonly mode: RuntimeMode["kind"];
  readonly baseURL: string;
  readonly artifactCollector: ArtifactCollector;
  readonly paths?: RuntimePaths;
  readonly seeded: BrowserRuntimeSeedResult;

  url(pathname?: string): string;
  requestJSON<T>(pathname: string, init?: RequestInit): Promise<T>;
  requestOperatorJSON?<T>(pathname: string, init?: RequestInit): Promise<T>;
  resolveWorkspace(rootDir: string): Promise<WorkspacePayload>;
  dispose(): Promise<void>;
}
export {
  browserAutomationOperatorFlowScenario,
  browserBridgeOperatorFlowScenario,
  browserSettingsOperatorFlowScenario,
  cleanupBrowserSettingsFixtures,
  browserNetworkOperatorFlowScenario,
  seedBrowserTasksOperatorFlow,
  seedBrowserBridgeOperatorFlow,
  seedBrowserAutomationOperatorFlow,
  seedBrowserSettingsFixtures,
  seedBrowserSandboxProfiles,
  triggerBrowserBridgeIngress,
  seedBrowserNetworkOperatorFlow,
  waitForSeedSessionActive,
  type BrowserAutomationOperatorFlowResult,
  type BrowserAutomationOperatorFlowSeed,
  type BrowserBridgeIngressResult,
  type BrowserBridgeIngressSeed,
  type BrowserBridgeOperatorFlowResult,
  type BrowserBridgeOperatorFlowSeed,
  type BrowserNetworkOperatorFlowResult,
  type BrowserNetworkOperatorFlowSeed,
  type BrowserSandboxProfileSeed,
  type BrowserSandboxProfilesResult,
  type BrowserSettingsFixturesResult,
  type BrowserSettingsFixturesSeed,
  type BrowserSettingsHookSeed,
  type BrowserSettingsMCPServerSeed,
  type BrowserSettingsProviderSeed,
  type BrowserTasksOperatorFlowResult,
  type BrowserTasksOperatorFlowSeed,
  type BridgeAdapterMarkerPaths,
  seedBrowserRuntimeHome,
  type BrowserRuntimeSeed,
  type BrowserRuntimeSeedResult,
  type WorkspacePayload,
} from "./runtime-seed";
interface RuntimeLaunchState {
  marketplaceCatalogServer?: Server;
  process: ChildProcessWithoutNullStreams;
  repoRoot: string;
  skillMarketplaceServer?: Server;
}

export async function createBrowserRuntime(
  options: BrowserRuntimeOptions
): Promise<BrowserRuntime> {
  const artifactCollector = await ArtifactCollector.create(options.artifactRootDir);
  const env = resolveBrowserRuntimeEnv(options.env);
  const mode = resolveRuntimeMode(env);

  if (mode.kind === "attach") {
    if ((options.seed?.mockAgents?.length ?? 0) > 0) {
      throw new Error(
        "attach mode does not support mock-agent runtime seeding; launch mode is required"
      );
    }
    await validateDaemonServedRuntime(mode.baseURL);
    const runtime = new ActiveBrowserRuntime({
      mode: "attach",
      baseURL: mode.baseURL,
      artifactCollector,
    });
    const seeded = await applyBrowserRuntimeSeed(runtime, options.seed);
    return runtime.withSeeded(seeded);
  }

  const repoRoot = await findRepoRoot();
  const binaryPath = await ensureDaemonBinary(repoRoot);
  const paths = await createRuntimePaths();
  const httpPort = await reserveFreePort();
  const boundHost = options.host ?? DEFAULT_HOST;
  const skillMarketplace = await startSkillMarketplaceServer(options.seed?.skillMarketplace);
  let marketplaceCatalog: Awaited<ReturnType<typeof startMarketplaceCatalogServer>> = undefined;
  let runtime: RuntimeLaunchState | undefined;

  try {
    marketplaceCatalog = await startMarketplaceCatalogServer(options.seed?.marketplaceCatalog);
    await seedBrowserRuntimeHome(
      {
        homeDir: paths.homeDir,
        repoRoot,
      },
      options.seed
    );
    await writeFile(
      paths.configFile,
      renderRuntimeConfig({
        extensionsAllowUnverified: options.extensionsAllowUnverified,
        host: boundHost,
        includeMockAgentProvider: (options.seed?.mockAgents?.length ?? 0) > 0,
        modelsDevEnabled: options.modelsDevEnabled,
        marketplaceCatalogBaseURL: marketplaceCatalog?.baseURL,
        networkEnabled: options.networkEnabled,
        port: httpPort,
        skillsMarketplaceBaseURL: skillMarketplace?.baseURL,
        socketPath: paths.daemonSocket,
        toolsExternalDefault: options.toolsExternalDefault,
      }),
      "utf8"
    );

    const runtimeEnv = await createRuntimeEnv(paths, binaryPath, repoRoot, env);
    runtime = startDaemonProcess(binaryPath, repoRoot, runtimeEnv, paths.daemonLog);
    runtime.marketplaceCatalogServer = marketplaceCatalog?.server;
    runtime.skillMarketplaceServer = skillMarketplace?.server;
    const baseURL = `http://${DEFAULT_HOST}:${httpPort}`;
    const requireHTTPAPIStatus = requiresHTTPAPIReadinessProbe(boundHost);
    await waitForRuntimeReady(
      baseURL,
      runtime.process,
      paths.daemonLog,
      options.readyTimeoutMs,
      requireHTTPAPIStatus
    );
    await validateDaemonServedRuntime(baseURL, requireHTTPAPIStatus);

    const activeRuntime = new ActiveBrowserRuntime({
      mode: "launch",
      baseURL,
      artifactCollector,
      paths,
      launchState: runtime,
    });
    const seeded = await applyBrowserRuntimeSeed(activeRuntime, options.seed);
    return activeRuntime.withSeeded(seeded);
  } catch (error) {
    return await cleanupFailedRuntimeLaunch(error, runtime, skillMarketplace, marketplaceCatalog);
  }
}

export function resolveBrowserRuntimeEnv(
  overrides: NodeJS.ProcessEnv | undefined,
  ambient: NodeJS.ProcessEnv = process.env
): NodeJS.ProcessEnv {
  if (overrides === undefined) {
    return ambient;
  }

  const resolved = { ...overrides };
  for (const name of RUNTIME_CONTROL_ENV_VARS) {
    if (resolved[name] === undefined && ambient[name] !== undefined) {
      resolved[name] = ambient[name];
    }
  }
  return resolved;
}

class ActiveBrowserRuntime implements BrowserRuntime {
  readonly mode: RuntimeMode["kind"];
  readonly baseURL: string;
  readonly artifactCollector: ArtifactCollector;
  readonly paths?: RuntimePaths;
  readonly seeded: BrowserRuntimeSeedResult;

  private readonly launchState?: RuntimeLaunchState;

  constructor(input: {
    mode: RuntimeMode["kind"];
    baseURL: string;
    artifactCollector: ArtifactCollector;
    paths?: RuntimePaths;
    seeded?: BrowserRuntimeSeedResult;
    launchState?: RuntimeLaunchState;
  }) {
    this.mode = input.mode;
    this.baseURL = input.baseURL;
    this.artifactCollector = input.artifactCollector;
    this.paths = input.paths;
    this.seeded = input.seeded ?? {};
    this.launchState = input.launchState;
  }

  withSeeded(seeded: BrowserRuntimeSeedResult): ActiveBrowserRuntime {
    return new ActiveBrowserRuntime({
      mode: this.mode,
      baseURL: this.baseURL,
      artifactCollector: this.artifactCollector,
      paths: this.paths,
      launchState: this.launchState,
      seeded,
    });
  }

  url(pathname = "/"): string {
    return runtimeURL(this.baseURL, pathname);
  }

  async requestJSON<T>(pathname: string, init?: RequestInit): Promise<T> {
    const headers = new Headers(init?.headers);
    if (!headers.has("content-type") && init?.body !== undefined) {
      headers.set("content-type", "application/json");
    }

    const response = await fetch(this.url(pathname), {
      ...init,
      headers,
    });
    if (!response.ok) {
      const payload = await response.text();
      throw new Error(`request ${pathname} failed with ${response.status}: ${payload.trim()}`);
    }
    return (await response.json()) as T;
  }

  async requestOperatorJSON<T>(pathname: string, init?: RequestInit): Promise<T> {
    if (!this.paths?.daemonSocket) {
      throw new Error(
        `operator request ${pathname} requires a launch-mode browser runtime with a daemon socket`
      );
    }

    return await requestJSONOverUnixSocket<T>(this.paths.daemonSocket, pathname, init);
  }

  async resolveWorkspace(rootDir: string): Promise<WorkspacePayload> {
    const payload = await this.requestJSON<{ workspace: WorkspacePayload }>(
      "/api/workspaces/resolve",
      {
        method: "POST",
        body: JSON.stringify(buildResolveWorkspaceRequest(rootDir)),
      }
    );
    return payload.workspace;
  }

  async dispose(): Promise<void> {
    if (this.launchState === undefined) {
      return;
    }

    try {
      if (this.paths === undefined) {
        await stopSpawnedDaemonProcess(this.launchState.process);
      } else {
        await stopBrowserDaemonProcess(this.launchState.process, {
          cliShim: this.paths.cliShim,
          homeDir: this.paths.homeDir,
          repoRoot: this.launchState.repoRoot,
        });
      }
    } finally {
      await Promise.all([
        closeSkillMarketplaceServer(this.launchState.skillMarketplaceServer),
        closeMarketplaceCatalogServer(this.launchState.marketplaceCatalogServer),
      ]);
    }
  }
}

async function validateDaemonServedRuntime(
  baseURL: string,
  requireHTTPAPIStatus = true
): Promise<void> {
  if (requireHTTPAPIStatus) {
    const statusResponse = await fetch(runtimeURL(baseURL, "/api/status"));
    if (!statusResponse.ok) {
      throw new Error(
        `runtime status probe failed for ${baseURL}: received ${statusResponse.status}`
      );
    }
  }

  const htmlResponse = await fetch(runtimeURL(baseURL, "/"));
  if (!htmlResponse.ok) {
    throw new Error(`root page probe failed for ${baseURL}: received ${htmlResponse.status}`);
  }
  const html = await htmlResponse.text();
  assertDaemonServedHTML(html, baseURL);
}

async function cleanupFailedRuntimeLaunch(
  cause: unknown,
  runtime: RuntimeLaunchState | undefined,
  skillMarketplace: SkillMarketplaceTestServer | undefined,
  marketplaceCatalog: Awaited<ReturnType<typeof startMarketplaceCatalogServer>>
): Promise<never> {
  const cleanupErrors: Error[] = [];
  if (runtime !== undefined) {
    try {
      await stopSpawnedDaemonProcess(runtime.process);
    } catch (error) {
      cleanupErrors.push(errorFromUnknown(error));
    }
  }

  try {
    await closeSkillMarketplaceServer(skillMarketplace?.server);
  } catch (error) {
    cleanupErrors.push(errorFromUnknown(error));
  }
  try {
    await closeMarketplaceCatalogServer(marketplaceCatalog?.server);
  } catch (error) {
    cleanupErrors.push(errorFromUnknown(error));
  }

  if (cleanupErrors.length > 0) {
    throw new AggregateError(
      [cause, ...cleanupErrors],
      "browser runtime launch failed and cleanup reported errors"
    );
  }
  throw cause;
}

function errorFromUnknown(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error));
}

async function waitForRuntimeReady(
  baseURL: string,
  child: ChildProcessWithoutNullStreams,
  daemonLogPath: string,
  timeoutMs = DEFAULT_READY_TIMEOUT_MS,
  requireHTTPAPIStatus = true
): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      const daemonLog = await safeReadFile(daemonLogPath);
      throw new Error(
        `daemon exited before readiness with code ${child.exitCode}\n${daemonLog}`.trim()
      );
    }

    try {
      const response = await fetch(runtimeURL(baseURL, requireHTTPAPIStatus ? "/api/status" : "/"));
      if (response.ok) {
        if (!requireHTTPAPIStatus) {
          assertDaemonServedHTML(await response.text(), baseURL);
        }
        return;
      }
    } catch {
      // Keep polling until the timeout or process exit.
    }

    await delay(DEFAULT_READY_POLL_MS);
  }

  const daemonLog = await safeReadFile(daemonLogPath);
  throw new Error(`timed out waiting for daemon readiness at ${baseURL}\n${daemonLog}`.trim());
}

async function ensureDaemonBinary(repoRoot: string): Promise<string> {
  const override = process.env[DAEMON_BINARY_ENV_VAR]?.trim();
  if (override) {
    return path.isAbsolute(override) ? override : path.resolve(repoRoot, override);
  }

  daemonBinaryPromise ??= buildDaemonBinary(repoRoot);
  return daemonBinaryPromise;
}

async function buildDaemonBinary(repoRoot: string): Promise<string> {
  await runCommand("bun", ["run", "build"], path.join(repoRoot, "web"));

  const buildDir = await mkdtemp(path.join(os.tmpdir(), "agh-playwright-build-"));
  const binaryName = process.platform === "win32" ? "agh.exe" : "agh";
  const binaryPath = path.join(buildDir, binaryName);
  await runCommand("go", ["build", "-o", binaryPath, "./cmd/agh"], repoRoot);
  return binaryPath;
}

async function createRuntimePaths(): Promise<RuntimePaths> {
  const homeDir = await mkdtemp(path.join(os.tmpdir(), "agh-playwright-home-"));
  const configFile = path.join(homeDir, "config.toml");
  const daemonSocket = path.join(
    os.tmpdir(),
    `agh-playwright-${process.pid}-${Date.now()}-${Math.round(Math.random() * 1000)}.sock`
  );
  const daemonLog = path.join(homeDir, "logs", "daemon-process.log");
  const cliShim = path.join(homeDir, "bin", process.platform === "win32" ? "agh.exe" : "agh");

  await mkdir(path.join(homeDir, "agents"), { recursive: true });
  await mkdir(path.join(homeDir, "skills"), { recursive: true });
  await mkdir(path.join(homeDir, "memory"), { recursive: true });
  await mkdir(path.join(homeDir, "sessions"), { recursive: true });
  await mkdir(path.join(homeDir, "logs"), { recursive: true });
  await mkdir(path.join(homeDir, "bin"), { recursive: true });

  return {
    homeDir,
    configFile,
    daemonSocket,
    daemonLog,
    cliShim,
  };
}

async function createRuntimeEnv(
  paths: RuntimePaths,
  binaryPath: string,
  repoRoot: string,
  env: NodeJS.ProcessEnv
): Promise<NodeJS.ProcessEnv> {
  await installCLIShim(binaryPath, paths.cliShim);

  return buildLaunchRuntimeEnv(repoRoot, {
    ...env,
    AGH_E2E_CLI_BIN: paths.cliShim,
    [DAEMON_BINARY_ENV_VAR]: binaryPath,
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: prependPath(path.dirname(paths.cliShim), env.PATH),
  });
}

async function installCLIShim(binaryPath: string, targetPath: string): Promise<void> {
  await mkdir(path.dirname(targetPath), { recursive: true });
  try {
    if (!existsSync(targetPath)) {
      await symlink(binaryPath, targetPath);
      return;
    }
  } catch {
    // Fall through to copy.
  }

  await copyFile(binaryPath, targetPath);
  if (process.platform !== "win32") {
    const { chmod } = await import("node:fs/promises");
    await chmod(targetPath, 0o755);
  }
}

function startDaemonProcess(
  binaryPath: string,
  repoRoot: string,
  env: NodeJS.ProcessEnv,
  daemonLogPath: string
): RuntimeLaunchState {
  const daemonLog = createWriteStream(daemonLogPath, { flags: "a" });
  const child = spawn(binaryPath, ["daemon", "start", "--foreground"], {
    cwd: repoRoot,
    env,
    stdio: "pipe",
  });

  child.stdout.pipe(daemonLog);
  child.stderr.pipe(daemonLog);
  child.once("close", () => {
    daemonLog.end();
  });

  return {
    process: child,
    repoRoot,
  };
}

async function requestJSONOverUnixSocket<T>(
  socketPath: string,
  pathname: string,
  init?: RequestInit
): Promise<T> {
  const http = await import("node:http");
  const headers = new Headers(init?.headers);
  const body = await normalizeRequestBody(init?.body);

  if (!headers.has("content-type") && body !== undefined) {
    headers.set("content-type", "application/json");
  }

  return await new Promise<T>((resolve, reject) => {
    const request = http.request(
      {
        headers: Object.fromEntries(headers.entries()),
        method: init?.method ?? "GET",
        path: pathname,
        socketPath,
      },
      response => {
        const chunks: Buffer[] = [];

        response.on("data", chunk => {
          chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
        });
        response.on("end", () => {
          const payload = Buffer.concat(chunks).toString("utf8");
          const statusCode = response.statusCode ?? 0;

          if (statusCode < 200 || statusCode >= 300) {
            reject(
              new Error(
                `request ${pathname} failed with ${statusCode}: ${payload.trim() || "unknown error"}`
              )
            );
            return;
          }

          try {
            resolve(JSON.parse(payload) as T);
          } catch (error) {
            reject(
              error instanceof Error
                ? new Error(`request ${pathname} returned invalid JSON: ${error.message}`)
                : error
            );
          }
        });
      }
    );

    request.on("error", reject);

    if (body !== undefined) {
      request.write(body);
    }

    request.end();
  });
}

async function normalizeRequestBody(body: RequestInit["body"]): Promise<string | undefined> {
  if (body === undefined || body === null) {
    return undefined;
  }
  if (typeof body === "string") {
    return body;
  }
  if (body instanceof URLSearchParams) {
    return body.toString();
  }
  if (body instanceof Uint8Array) {
    return Buffer.from(body).toString("utf8");
  }
  if (body instanceof ArrayBuffer) {
    return Buffer.from(body).toString("utf8");
  }

  throw new Error("unsupported request body for unix-socket JSON request");
}

async function reserveFreePort(): Promise<number> {
  const net = await import("node:net");

  return await new Promise<number>((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, DEFAULT_HOST, () => {
      const address = server.address();
      if (address === null || typeof address === "string") {
        server.close(() => reject(new Error("failed to resolve free TCP port")));
        return;
      }

      const { port } = address;
      server.close(error => {
        if (error) {
          reject(error);
          return;
        }
        resolve(port);
      });
    });
    server.on("error", reject);
  });
}

async function runCommand(command: string, args: string[], cwd: string): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    const child = spawn(command, args, {
      cwd,
      env: process.env,
      stdio: "pipe",
    });

    let stdout = "";
    let stderr = "";
    child.stdout.on("data", chunk => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", chunk => {
      stderr += chunk.toString();
    });
    child.on("error", reject);
    child.on("close", code => {
      if (code === 0) {
        resolve();
        return;
      }

      reject(
        new Error(
          `${command} ${args.join(" ")} failed with exit code ${code}\n${stdout}${stderr}`.trim()
        )
      );
    });
  });
}

async function findRepoRoot(): Promise<string> {
  let current = path.dirname(fileURLToPath(import.meta.url));
  while (current !== path.dirname(current)) {
    if (existsSync(path.join(current, "go.mod"))) {
      return current;
    }
    current = path.dirname(current);
  }

  throw new Error("failed to locate repository root for Playwright runtime");
}

async function safeReadFile(filePath: string): Promise<string> {
  try {
    return (await readFile(filePath, "utf8")).trim();
  } catch {
    return "";
  }
}

function delay(durationMs: number): Promise<void> {
  return new Promise(resolve => {
    setTimeout(resolve, durationMs);
  });
}
