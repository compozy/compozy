import { createHash } from "node:crypto";
import { createServer, type Server } from "node:http";
import { readFile, rename, writeFile } from "node:fs/promises";
import { basename, join } from "node:path";

import { expect, test, type DesktopInstance, type PackagedCopy } from "../fixtures";

interface UpdateFixture {
  readonly asset: string;
  readonly baseline_app_image?: string;
  readonly current_version: string;
  readonly directory: string;
  readonly manifest: string;
  readonly next_version: string;
}

interface MockFeed {
  readonly url: string;
  close(): Promise<void>;
}

const repositoryRoot = join(import.meta.dir, "../../../..");

async function updateFixture(): Promise<UpdateFixture> {
  const value: unknown = JSON.parse(
    await readFile(join(import.meta.dir, "../../../.artifacts/e2e-update-fixture.json"), "utf8")
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

async function startMockFeed(fixture: UpdateFixture): Promise<MockFeed> {
  const allowed = new Set([fixture.asset, fixture.manifest]);
  const server = createServer((request, response) => {
    const requested = basename(new URL(request.url ?? "/", "http://127.0.0.1").pathname);
    if (!allowed.has(requested)) {
      response.writeHead(404).end();
      return;
    }
    void readFile(join(fixture.directory, requested)).then(
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
      await closeServer(server);
    },
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
  desktop: DesktopInstance,
  fixture: UpdateFixture,
  digest: string
): Promise<void> {
  const process = Bun.spawn(
    [
      "go",
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
    { cwd: repositoryRoot, env: desktop.environment, stdout: "pipe", stderr: "pipe" }
  );
  const [exitCode, stderr] = await Promise.all([
    process.exited,
    new Response(process.stderr).text(),
  ]);
  if (exitCode !== 0) throw new Error(stderr.trim() || "The app stage fixture failed.");
}

function appUpdateSnapshot(
  fixture: UpdateFixture,
  operation: Record<string, unknown> | null,
  available: boolean
): Record<string, unknown> {
  const appOperation = operation?.app as Record<string, unknown> | undefined;
  return {
    aggregate: operation ? "applying" : available ? "available" : "up-to-date",
    operation: operation
      ? {
          id: operation.operation_id,
          revision: operation.revision,
          targets: operation.targets,
          active_target: operation.active_target,
          phase: appOperation?.phase,
          percent: operation.percent,
          holder: operation.holder,
          waiting: operation.waiting,
          started_at: operation.started_at,
        }
      : null,
    runtime: {
      status: "up-to-date",
      install_method: "desktop-app",
      managed: false,
      current_version: fixture.current_version,
      latest_version: fixture.current_version,
      daemon_restarted: false,
      message: "CompozyOS runtime is up to date.",
    },
    app: {
      status: operation ? "applying" : available ? "available" : "up-to-date",
      running: true,
      current_version: fixture.current_version,
      latest_version: available ? fixture.next_version : fixture.current_version,
      message: available
        ? `CompozyOS app ${fixture.next_version} is available.`
        : "CompozyOS app is up to date.",
    },
  };
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
  const contents = await readFile(join(home, "logs", "update-history.jsonl"), "utf8");
  return contents
    .split("\n")
    .filter(Boolean)
    .map(line => JSON.parse(line) as Record<string, unknown>);
}

async function waitForShellExit(desktop: DesktopInstance): Promise<void> {
  if (desktop.app.process().exitCode !== null) return;
  await new Promise<void>((resolveExit, reject) => {
    const timeout = setTimeout(
      () => reject(new Error("The updater did not restart the shell.")),
      60_000
    );
    desktop.app.process().once("exit", () => {
      clearTimeout(timeout);
      resolveExit();
    });
  });
}

function updaterEnvironment(fixture: UpdateFixture): NodeJS.ProcessEnv {
  if (process.platform !== "linux") return {};
  if (!fixture.baseline_app_image) throw new Error("The baseline AppImage is missing.");
  return { APPIMAGE: fixture.baseline_app_image };
}

test("E2E-016: Settings accepts a feed offer and the recorded app asset restarts verified", async ({
  copyPackagedExecutable,
  launchDesktop,
}) => {
  const fixture = await updateFixture();
  const packaged = await copyPackagedExecutable();
  const feed = await startMockFeed(fixture);
  try {
    await configureFeed(packaged, feed.url);
    const digest = await artifactDigest(fixture);
    const desktop = await launchDesktop({
      executablePath: packaged.executablePath,
      environment: updaterEnvironment(fixture),
    });
    const product = await desktop.product();
    let available = false;
    let staged = false;
    await product.route("**/api/settings/update*", async route => {
      const request = route.request();
      const pathname = new URL(request.url()).pathname;
      if (request.method() === "GET" && pathname === "/api/settings/update") {
        let current: Record<string, unknown> | null = null;
        if (staged) {
          try {
            current = JSON.parse(
              await readFile(join(desktop.home, "update-operation.json"), "utf8")
            ) as Record<string, unknown>;
          } catch (error) {
            if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) {
              throw error;
            }
          }
        }
        await route.fulfill({ json: appUpdateSnapshot(fixture, current, available) });
        return;
      }
      if (request.method() === "POST" && pathname === "/api/settings/update/apply") {
        await stageAppOperation(desktop, fixture, digest);
        staged = true;
        await route.fulfill({
          json: {
            target: "app",
            status: "accepted",
            operation_id: "e2e-app-update",
            message: "Update accepted.",
            holder: null,
          },
        });
        return;
      }
      await route.continue();
    });
    await product.reload();
    await expect(product.getByRole("button", { name: "Update available" })).toHaveCount(0);
    available = true;
    await product.reload();
    await product.getByRole("button", { name: "Update available" }).click();
    await product.getByTestId("settings-page-general-update-apply-app").click();
    await expect(product.getByText(/Downloading|Verifying|Installing/u)).toBeVisible();
    await waitForShellExit(desktop);
    await expect
      .poll(async () => (await readAppRecord(desktop.home)).app_version, { timeout: 60_000 })
      .toBe(fixture.next_version);
    await expect
      .poll(async () => (await history(desktop.home)).at(-1)?.outcome, { timeout: 30_000 })
      .toBe("updated");
  } finally {
    await feed.close();
  }
});

test("E2E-017: expired installer handoff records old-version truth and a fresh retry succeeds", async ({
  copyPackagedExecutable,
  launchDesktop,
}) => {
  const fixture = await updateFixture();
  const packaged = await copyPackagedExecutable();
  const feed = await startMockFeed(fixture);
  try {
    await configureFeed(packaged, feed.url);
    const digest = await artifactDigest(fixture);
    const desktop = await launchDesktop({
      executablePath: packaged.executablePath,
      environment: updaterEnvironment(fixture),
      prepare: async ({ home }) =>
        await seedOperation(home, operation(fixture, digest, { expiredHandoff: true })),
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
    await feed.close();
  }
});
