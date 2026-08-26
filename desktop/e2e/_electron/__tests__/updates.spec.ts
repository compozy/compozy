import { createHash } from "node:crypto";
import { createServer, type Server } from "node:http";
import { copyFile, mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { basename, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test, type DesktopInstance, type PackagedCopy } from "../fixtures";
import { completeOnboarding, readDiagnosticFile } from "../helpers";
import { runCommand } from "../process";

interface UpdateFixture {
  readonly asset: string;
  readonly baseline_app_image?: string;
  readonly baseline_executable?: string;
  readonly current_version: string;
  readonly directory: string;
  readonly manifest: string;
  readonly next_version: string;
}

interface MockFeed {
  readonly url: string;
  close(): Promise<void>;
  releaseAsset(): void;
}

type RestoreEnvironment = () => Promise<void>;

const testDirectory = fileURLToPath(new URL(".", import.meta.url));
const repositoryRoot = join(testDirectory, "../../../..");

async function updateFixture(): Promise<UpdateFixture> {
  const value: unknown = JSON.parse(
    await readFile(join(testDirectory, "../../../.artifacts/e2e-update-fixture.json"), "utf8")
  );
  if (!value || typeof value !== "object") throw new Error("The update fixture is invalid.");
  for (const field of [
    "asset",
    "current_version",
    "directory",
    "manifest",
    "next_version",
  ] as const) {
    if (typeof Reflect.get(value, field) !== "string") {
      throw new Error(`The update fixture field ${field} is invalid.`);
    }
  }
  return value as UpdateFixture;
}

async function listen(server: Server): Promise<number> {
  return await new Promise((resolveListen, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", reject);
      const address = server.address();
      if (!address || typeof address === "string") {
        reject(new Error("The mock update feed did not bind a TCP port."));
        return;
      }
      resolveListen(address.port);
    });
  });
}

async function closeServer(server: Server): Promise<void> {
  await new Promise<void>((resolveClose, reject) => {
    server.close(error => (error ? reject(error) : resolveClose()));
  });
}

async function configureRelaunchHome(home: string): Promise<RestoreEnvironment> {
  if (process.platform !== "darwin") return () => Promise.resolve();
  const previous = await runCommand("launchctl", ["getenv", "COMPOZY_HOME"]);
  if (previous.exitCode !== 0) {
    throw new Error(previous.stderr.trim() || "Could not read the launch-session COMPOZY_HOME.");
  }
  const configured = await runCommand("launchctl", ["setenv", "COMPOZY_HOME", home]);
  if (configured.exitCode !== 0) {
    throw new Error(
      configured.stderr.trim() || "Could not isolate the native updater relaunch home."
    );
  }
  return async () => {
    const value = previous.stdout.trim();
    const restored = await runCommand(
      "launchctl",
      value ? ["setenv", "COMPOZY_HOME", value] : ["unsetenv", "COMPOZY_HOME"]
    );
    if (restored.exitCode !== 0) {
      throw new Error(
        restored.stderr.trim() || "Could not restore the launch-session COMPOZY_HOME."
      );
    }
  };
}

async function closeFixture(feed: MockFeed, restoreEnvironment: RestoreEnvironment): Promise<void> {
  const results = await Promise.allSettled([feed.close(), restoreEnvironment()]);
  const errors = results.flatMap(result => (result.status === "rejected" ? [result.reason] : []));
  if (errors.length === 1) throw errors[0];
  if (errors.length > 1) throw new AggregateError(errors, "The update fixture cleanup failed.");
}

async function startMockFeed(
  fixture: UpdateFixture,
  options: { readonly holdAsset?: boolean } = {}
): Promise<MockFeed> {
  const allowed = new Set([fixture.asset, fixture.manifest]);
  let releaseAsset = () => {};
  const assetReady = options.holdAsset
    ? new Promise<void>(resolveAsset => {
        releaseAsset = resolveAsset;
      })
    : Promise.resolve();
  const server = createServer((request, response) => {
    const requested = basename(new URL(request.url ?? "/", "http://127.0.0.1").pathname);
    if (!allowed.has(requested)) {
      response.writeHead(404).end();
      return;
    }
    void (requested === fixture.asset ? assetReady : Promise.resolve())
      .then(async () => await readFile(join(fixture.directory, requested)))
      .then(
        payload => {
          response.writeHead(200, {
            "content-length": payload.byteLength,
            "content-type": requested.endsWith(".yml") ? "text/yaml" : "application/octet-stream",
          });
          response.end(payload);
        },
        error => {
          response.writeHead(500).end(error instanceof Error ? error.message : "Feed read failed.");
        }
      );
  });
  const port = await listen(server);
  return {
    url: `http://127.0.0.1:${port}/`,
    async close() {
      releaseAsset();
      await closeServer(server);
    },
    releaseAsset,
  };
}

async function configureFeed(packaged: PackagedCopy, feedURL: string): Promise<void> {
  await writeFile(
    join(packaged.resourcesPath, "app-update.yml"),
    `provider: generic\nurl: ${feedURL}\nupdaterCacheDirName: compozyos-updater\n`,
    { mode: 0o600 }
  );
}

async function artifactDigest(fixture: UpdateFixture): Promise<string> {
  return `sha256:${createHash("sha256")
    .update(await readFile(join(fixture.directory, fixture.asset)))
    .digest("hex")}`;
}

async function stageAppOperation(
  desktop: Pick<DesktopInstance, "environment" | "home">,
  fixture: UpdateFixture,
  digest: string
): Promise<void> {
  const stageResult = await runCommand(
    "go",
    [
      "run",
      "-tags",
      "e2e",
      "./desktop/e2e/fixtures/appstage",
      "--home",
      desktop.home,
      "--from-version",
      fixture.current_version,
      "--to-version",
      fixture.next_version,
      "--asset",
      fixture.asset,
      "--digest",
      digest,
    ],
    { cwd: repositoryRoot, env: desktop.environment }
  );
  if (stageResult.exitCode !== 0) {
    throw new Error(stageResult.stderr.trim() || "The app stage fixture failed.");
  }
}

async function seedUpdateCache(home: string, fixture: UpdateFixture): Promise<void> {
  const cache = join(home, "cache");
  await mkdir(cache, { recursive: true, mode: 0o700 });
  const now = new Date().toISOString();
  await writeFile(
    join(cache, "update-state-stable.json"),
    `${JSON.stringify(
      {
        latest_version: fixture.next_version,
        release_url: `https://example.test/releases/v${fixture.next_version}`,
        published_at: now,
        checked_at: now,
        assets: [{ Name: fixture.asset, DownloadURL: `https://example.test/${fixture.asset}` }],
      },
      null,
      2
    )}\n`,
    { mode: 0o600 }
  );
}

function operation(
  fixture: UpdateFixture,
  digest: string,
  options: { readonly expiredHandoff?: boolean; readonly failures?: number } = {}
): Record<string, unknown> {
  const now = new Date();
  const expiredHandoff = options.expiredHandoff === true;
  return {
    schema_version: 1,
    operation_id: `e2e-app-${crypto.randomUUID()}`,
    requested_by: "web",
    revision: 3,
    targets: ["app"],
    ...(expiredHandoff ? { active_target: "app" } : {}),
    percent: expiredHandoff ? 100 : -1,
    app: {
      from_version: fixture.current_version,
      to_version: fixture.next_version,
      release_tag: `v${fixture.next_version}`,
      asset: fixture.asset,
      digest,
      attempt_id: crypto.randomUUID(),
      phase: expiredHandoff ? "installer-handoff" : "staged",
      consecutive_failures: options.failures ?? 0,
      ...(expiredHandoff
        ? { watchdog_deadline: new Date(now.getTime() - 1_000).toISOString() }
        : {}),
    },
    holder: null,
    waiting: expiredHandoff ? "" : "waiting-for-app",
    deadline: new Date(now.getTime() + 30 * 60_000).toISOString(),
    started_at: now.toISOString(),
    updated_at: now.toISOString(),
  };
}

async function seedOperation(home: string, value: Record<string, unknown>): Promise<void> {
  const path = join(home, "update-operation.json");
  const temporary = `${path}.e2e-${crypto.randomUUID()}`;
  await writeFile(temporary, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
  await rename(temporary, path);
}

async function readAppRecord(home: string): Promise<Record<string, unknown>> {
  return JSON.parse(await readFile(join(home, "app.json"), "utf8")) as Record<string, unknown>;
}

async function history(home: string): Promise<readonly Record<string, unknown>[]> {
  let contents: string;
  try {
    contents = await readFile(join(home, "logs", "update-history.jsonl"), "utf8");
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return [];
    throw error;
  }
  return contents
    .split("\n")
    .filter(Boolean)
    .map(line => JSON.parse(line) as Record<string, unknown>);
}

async function waitForShellExit(desktop: DesktopInstance): Promise<void> {
  const deadline = Date.now() + 60_000;
  while (Date.now() < deadline) {
    if (desktop.shellProcess.exitCode !== null) return;
    await new Promise(resolveDelay => setTimeout(resolveDelay, 100));
  }
  const [operationState, desktopLog, bootstrapLog] = await Promise.all([
    readDiagnosticFile(join(desktop.home, "update-operation.json")),
    readDiagnosticFile(join(desktop.home, "logs", "desktop.log")),
    readDiagnosticFile(join(desktop.home, "logs", "desktop-bootstrap.jsonl")),
  ]);
  throw new Error(
    `The updater did not restart the shell.\nOperation:\n${operationState}\nDesktop log:\n${desktopLog}\nBootstrap log:\n${bootstrapLog}`
  );
}

async function updaterEnvironment(
  fixture: UpdateFixture,
  packaged: PackagedCopy
): Promise<NodeJS.ProcessEnv> {
  if (process.platform !== "linux") return {};
  if (!fixture.baseline_app_image) throw new Error("The baseline AppImage is missing.");
  const appImage = join(dirname(packaged.executablePath), basename(fixture.baseline_app_image));
  await copyFile(fixture.baseline_app_image, appImage);
  return { APPIMAGE: appImage, APPIMAGE_EXTRACT_AND_RUN: "1" };
}

test("E2E-016: the real Settings API projects a staged app asset through verified restart", async ({
  copyPackagedExecutable,
  launchDesktop,
}) => {
  const fixture = await updateFixture();
  const packaged = await copyPackagedExecutable(fixture.baseline_executable);
  const feed = await startMockFeed(fixture, { holdAsset: true });
  let restoreEnvironment: RestoreEnvironment = () => Promise.resolve();
  try {
    await configureFeed(packaged, feed.url);
    const digest = await artifactDigest(fixture);
    const desktop = await launchDesktop({
      executablePath: packaged.executablePath,
      environment: await updaterEnvironment(fixture, packaged),
      prepare: async context => {
        restoreEnvironment = await configureRelaunchHome(context.home);
        await seedUpdateCache(context.home, fixture);
      },
    });
    const product = await desktop.product();
    await completeOnboarding(product);
    await expect
      .poll(async () => {
        return await product.evaluate(async () => {
          const response = await fetch("/api/settings/update");
          if (!response.ok) throw new Error(`Update status failed with ${response.status}.`);
          const status = (await response.json()) as { app?: Record<string, unknown> };
          return status.app;
        });
      })
      .toMatchObject({
        status: "available",
        current_version: fixture.current_version,
        latest_version: fixture.next_version,
      });
    await product.getByRole("button", { name: "Update available" }).click();
    await stageAppOperation(desktop, fixture, digest);
    await product.reload({ waitUntil: "domcontentloaded" });
    await expect(product.getByRole("group", { name: "App" }).getByRole("status")).toContainText(
      "download"
    );
    feed.releaseAsset();
    await waitForShellExit(desktop);
    await expect
      .poll(async () => (await readAppRecord(desktop.home)).app_version, { timeout: 60_000 })
      .toBe(fixture.next_version);
    await expect
      .poll(async () => (await history(desktop.home)).at(-1)?.outcome, { timeout: 30_000 })
      .toBe("updated");
  } finally {
    await closeFixture(feed, restoreEnvironment);
  }
});

test("E2E-017: expired installer handoff records old-version truth and a fresh retry succeeds", async ({
  copyPackagedExecutable,
  launchDesktop,
}) => {
  const fixture = await updateFixture();
  const packaged = await copyPackagedExecutable(fixture.baseline_executable);
  const feed = await startMockFeed(fixture);
  let restoreEnvironment: RestoreEnvironment = () => Promise.resolve();
  try {
    await configureFeed(packaged, feed.url);
    const digest = await artifactDigest(fixture);
    const desktop = await launchDesktop({
      executablePath: packaged.executablePath,
      environment: await updaterEnvironment(fixture, packaged),
      prepare: async ({ home }) => {
        restoreEnvironment = await configureRelaunchHome(home);
        await seedUpdateCache(home, fixture);
        await seedOperation(home, operation(fixture, digest, { expiredHandoff: true }));
      },
    });
    const product = await desktop.product();
    await expect
      .poll(async () => (await history(desktop.home)).at(-1)?.outcome, { timeout: 30_000 })
      .toBe("failed");
    expect((await readAppRecord(desktop.home)).app_version).toBe(fixture.current_version);
    const failedApp = (await history(desktop.home)).at(-1)?.app as Record<string, unknown>;
    expect(failedApp.consecutive_failures).toBe(1);
    const background = await product.evaluate(async () => {
      const response = await fetch("/api/settings/update");
      if (!response.ok) throw new Error(`Update status failed with ${response.status}.`);
      return (await response.json()) as Record<string, unknown>;
    });
    expect(background.app).toMatchObject({ status: "failed" });
    expect((background.app as Record<string, unknown>).message).toMatch(/paused.*Retry manually/u);

    await seedOperation(desktop.home, operation(fixture, digest, { failures: 1 }));
    await waitForShellExit(desktop);
    await expect
      .poll(async () => (await readAppRecord(desktop.home)).app_version, { timeout: 60_000 })
      .toBe(fixture.next_version);
    await expect
      .poll(async () => (await history(desktop.home)).at(-1)?.outcome, { timeout: 30_000 })
      .toBe("updated");
  } finally {
    await closeFixture(feed, restoreEnvironment);
  }
});
