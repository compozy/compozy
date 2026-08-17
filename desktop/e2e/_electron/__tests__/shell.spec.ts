import { cp, chmod, mkdir, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { extract } from "tar";

import {
  availableLoopbackPort,
  expect,
  productionPackagedExecutable,
  test,
  type DesktopInstance,
} from "../fixtures";
import { runCommand, startCommand, type RunningCommand } from "../process";
import { completeOnboarding } from "../helpers";

const testDirectory = fileURLToPath(new URL(".", import.meta.url));
const repositoryRoot = resolve(testDirectory, "../../../..");

async function jsonCommand(
  desktop: DesktopInstance,
  arguments_: readonly string[]
): Promise<Record<string, unknown>> {
  return JSON.parse((await desktop.cli(arguments_)).stdout) as Record<string, unknown>;
}

async function bootstrapEvents(home: string): Promise<readonly Record<string, unknown>[]> {
  const contents = await readFile(join(home, "logs", "desktop-bootstrap.jsonl"), "utf8");
  return contents
    .split("\n")
    .filter(Boolean)
    .map(line => JSON.parse(line) as Record<string, unknown>);
}

async function corruptRuntimeBundle(context: {
  readonly bundleRuntimePath: string;
  readonly environment: Record<string, string>;
  readonly home: string;
  readonly resourcesPath: string;
}): Promise<void> {
  const bundleRoot = join(context.home, "corrupt-runtime-bundle");
  await mkdir(bundleRoot, { recursive: true, mode: 0o700 });
  const runtimeName = process.platform === "win32" ? "compozy.exe" : "compozy";
  await Promise.all([
    cp(context.bundleRuntimePath, join(bundleRoot, runtimeName)),
    cp(
      join(context.resourcesPath, "runtime", "runtime-manifest.json"),
      join(bundleRoot, "runtime-manifest.json")
    ),
  ]);
  await writeFile(join(bundleRoot, runtimeName), "corrupted-runtime");
  context.environment.COMPOZY_DESKTOP_BUNDLE_ROOT = bundleRoot;
}

async function buildRuntimeFixture(
  output: string,
  version: string,
  environment: NodeJS.ProcessEnv
): Promise<void> {
  await mkdir(resolve(output, ".."), { recursive: true, mode: 0o700 });
  const ldflags = [
    `-X github.com/compozy/compozy/internal/version.Version=${version}`,
    "-X github.com/compozy/compozy/internal/version.Commit=e2e",
    "-X github.com/compozy/compozy/internal/version.BuildDate=2026-08-16T00:00:00Z",
    "-X github.com/compozy/compozy/internal/version.MinAppVersion=0.0.0",
  ].join(" ");
  const build = await runCommand(
    "go",
    ["build", "-trimpath", "-ldflags", ldflags, "-o", output, "./cmd/compozy"],
    { cwd: repositoryRoot, env: environment }
  );
  if (build.exitCode !== 0) {
    throw new Error(build.stderr.trim() || "Fixture build failed.");
  }
  if (process.platform !== "win32") await chmod(output, 0o700);
}

async function startRuntimeFixture(runtime: string, environment: NodeJS.ProcessEnv): Promise<void> {
  const started = await runCommand(runtime, ["daemon", "start"], { env: environment });
  if (started.exitCode !== 0) {
    throw new Error(started.stderr.trim() || "Fixture start failed.");
  }
}

async function buildRuntimeUpdateCoordinator(
  output: string,
  environment: NodeJS.ProcessEnv
): Promise<void> {
  const build = await runCommand(
    "go",
    ["build", "-tags", "e2e", "-o", output, "./desktop/e2e/fixtures/runtimeupdate"],
    { cwd: repositoryRoot, env: environment }
  );
  if (build.exitCode !== 0) {
    throw new Error(build.stderr.trim() || "Update coordinator fixture build failed.");
  }
  if (process.platform !== "win32") await chmod(output, 0o700);
}

async function seedRuntimeUpdateCache(home: string): Promise<void> {
  const cache = join(home, "cache");
  await mkdir(cache, { recursive: true, mode: 0o700 });
  const now = new Date().toISOString();
  await writeFile(
    join(cache, "update-state-stable.json"),
    `${JSON.stringify(
      {
        latest_version: "0.3.1",
        release_url: "https://example.test/releases/v0.3.1",
        published_at: now,
        checked_at: now,
        assets: [{ Name: "fixture", DownloadURL: "https://example.test/fixture" }],
      },
      null,
      2
    )}\n`,
    { mode: 0o600 }
  );
}

async function waitForUpdateOperation(home: string): Promise<void> {
  const operationPath = join(home, "update-operation.json");
  const deadline = Date.now() + 5_000;
  while (Date.now() < deadline) {
    try {
      await stat(operationPath);
      return;
    } catch (error) {
      if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) throw error;
    }
    await new Promise(resolveDelay => setTimeout(resolveDelay, 25));
  }
  throw new Error("The runtime update fixture did not create its durable operation in time.");
}

async function invalidAppOpenCode(desktop: DesktopInstance, target: string): Promise<string> {
  try {
    await desktop.cli(["app", "open", target, "-o", "json"]);
  } catch (error) {
    const payload = JSON.parse(error instanceof Error ? error.message : String(error)) as {
      error?: { code?: unknown };
    };
    return typeof payload.error?.code === "string" ? payload.error.code : "";
  }
  throw new Error(`Invalid app target ${target} unexpectedly succeeded.`);
}

test("E2E-001 E2E-002: first run provisions offline and exposes every boot phase", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop({
    environment: {
      HTTP_PROXY: "http://127.0.0.1:9",
      HTTPS_PROXY: "http://127.0.0.1:9",
      NO_PROXY: "127.0.0.1,localhost,::1",
    },
  });
  const product = await desktop.product();
  await expect(product).toHaveTitle(/CompozyOS/u);
  const events = await bootstrapEvents(desktop.home);
  const completedPhases = events
    .filter(event => event.status === "completed")
    .map(event => event.phase);
  expect(completedPhases).toEqual(["provision", "ready"]);
  const firstSeenPhases = events
    .map(event => event.phase)
    .filter((phase, index, phases) => phases.indexOf(phase) === index);
  expect(firstSeenPhases).toEqual(["resolve", "provision", "start", "ready"]);
  const status = await jsonCommand(desktop, ["app", "status", "-o", "json"]);
  expect(status).toMatchObject({ installed: true, running: true, state: "product" });
  expect(status.runtime).toMatchObject({ attached: true, owned: true });
  expect(
    (
      await stat(
        join(desktop.home, "bin", process.platform === "win32" ? "compozy.exe" : "compozy")
      )
    ).isFile()
  ).toBe(true);
});

test("E2E-002: corrupt bundled runtime fails before provisioning and remains retryable", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop({ prepare: corruptRuntimeBundle });
  await expect(
    desktop.boot.getByRole("heading", { name: "CompozyOS could not start" })
  ).toBeVisible();
  await expect(desktop.boot.getByText(/integrity check|Reinstall the app/u)).toBeVisible();
  await expect(desktop.boot.getByRole("button", { name: "Retry operation" })).toBeVisible();
  await expect(
    stat(join(desktop.home, "bin", process.platform === "win32" ? "compozy.exe" : "compozy"))
  ).rejects.toThrow();
});

test("E2E-003: runtime below the minimum stays on version-skew guidance", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop({
    prepare: async ({ environment, home }) => {
      const runtime = join(home, "bin", process.platform === "win32" ? "compozy.exe" : "compozy");
      await buildRuntimeFixture(runtime, "0.3.0-beta.1", environment);
      await startRuntimeFixture(runtime, environment);
    },
  });
  await expect(
    desktop.boot.getByRole("heading", { name: "CompozyOS needs an update" })
  ).toBeVisible();
  await expect(desktop.boot.getByText(/Runtime 0\.3\.0-beta\.1.*Repair or update/u)).toBeVisible();
  await expect(desktop.boot.getByRole("button", { name: "Retry operation" })).toHaveCount(0);
});

test("E2E-004: launch bursts reuse one window and deliver the last deep link", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await Promise.all([
    desktop.spawnSecondary([]),
    desktop.spawnSecondary(["compozyos://open/tasks"]),
  ]);
  await desktop.spawnSecondary(["compozyos://open/settings/general"]);
  await expect(product).toHaveURL(/\/settings\/general(?:\?|$)/u);
  const windows = await desktop.app.evaluate(
    ({ BrowserWindow }) =>
      BrowserWindow.getAllWindows().filter(window => {
        const url = window.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      }).length
  );
  expect(windows).toBe(1);
  expect(
    await desktop.app.evaluate(({ BrowserWindow }) =>
      BrowserWindow.getAllWindows().some(window => window.isVisible())
    )
  ).toBe(true);
});

test("E2E-005 E2E-007: relaunch attaches, stopped runtime starts once, and shell quit leaves it alive", async ({
  launchDesktop,
}) => {
  const first = await launchDesktop();
  await first.product();
  const initial = await jsonCommand(first, ["status", "-o", "json"]);
  await first.closeShell();
  const alive = await jsonCommand(first, ["status", "-o", "json"]);
  const initialDaemon = initial.daemon as Record<string, unknown>;
  expect(alive).toMatchObject({
    daemon: { status: "running", pid: initialDaemon.pid },
  });

  const attached = await launchDesktop({ home: first.home });
  await attached.product();
  const attachEvents = await bootstrapEvents(first.home);
  expect(attachEvents.at(-1)).toMatchObject({ phase: "ready", resolution: "attach" });
  await attached.closeShell();
  await attached.cli(["daemon", "stop"]);

  const started = await launchDesktop({ home: first.home });
  await started.product();
  const startEvents = await bootstrapEvents(first.home);
  expect(startEvents.at(-1)).toMatchObject({ phase: "ready", resolution: "start" });
  const restarted = await jsonCommand(started, ["status", "-o", "json"]);
  expect((restarted.daemon as Record<string, unknown>).pid).not.toBe(initialDaemon.pid);
});

test("E2E-006: bounded startup failure exposes retry, logs, quit, and recovers after repair", async ({
  launchDesktop,
}) => {
  const runtimeName = process.platform === "win32" ? "compozy.exe" : "compozy";
  const desktop = await launchDesktop({
    prepare: async ({ home }) => {
      const runtime = join(home, "bin", runtimeName);
      await mkdir(resolve(runtime, ".."), { recursive: true, mode: 0o700 });
      await writeFile(runtime, "#!/bin/sh\nexit 1\n", { mode: 0o700 });
    },
  });
  await expect(
    desktop.boot.getByRole("heading", { name: "CompozyOS could not start" })
  ).toBeVisible();
  for (const name of ["Retry operation", "Open logs", "Quit", "Load diagnostics"]) {
    await expect(desktop.boot.getByRole("button", { name })).toBeVisible();
  }
  const events = await bootstrapEvents(desktop.home);
  expect(
    events.filter(event => event.phase === "start" && event.status === "started")
  ).toHaveLength(5);
  await cp(desktop.bundleRuntimePath, join(desktop.home, "bin", runtimeName));
  if (process.platform !== "win32") await chmod(join(desktop.home, "bin", runtimeName), 0o700);
  await desktop.boot.getByRole("button", { name: "Retry operation" }).click();
  await expect(await desktop.product()).toHaveTitle(/CompozyOS/u);
});

test("E2E-008: bounds, maximized state, and off-screen recovery survive relaunch", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  await desktop.product();
  await desktop.app.evaluate(({ BrowserWindow }) => {
    const window = BrowserWindow.getAllWindows().find(candidate => {
      const url = candidate.webContents.getURL();
      return url.startsWith("http://") || url.startsWith("https://");
    });
    if (!window) throw new Error("Product window missing.");
    window.setBounds({ x: 80, y: 70, width: 960, height: 720 });
    window.maximize();
  });
  await desktop.closeShell();
  const maximized = await launchDesktop({ home: desktop.home });
  await maximized.product();
  expect(
    await maximized.app.evaluate(({ BrowserWindow }) =>
      BrowserWindow.getAllWindows().some(window => window.isMaximized())
    )
  ).toBe(true);
  await maximized.closeShell();
  await writeFile(
    join(desktop.home, "desktop-window.json"),
    `${JSON.stringify({ x: 999999, y: 999999, width: 960, height: 720, maximized: false, zoom_level: 0 })}\n`,
    { mode: 0o600 }
  );
  const clamped = await launchDesktop({ home: desktop.home });
  await clamped.product();
  expect(
    await clamped.app.evaluate(({ BrowserWindow, screen }) => {
      const window = BrowserWindow.getAllWindows().find(candidate => {
        const url = candidate.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      });
      if (!window) return false;
      const bounds = window.getBounds();
      const area = screen.getDisplayMatching(bounds).workArea;
      return (
        bounds.x >= area.x &&
        bounds.y >= area.y &&
        bounds.x + bounds.width <= area.x + area.width &&
        bounds.y + bounds.height <= area.y + area.height
      );
    })
  ).toBe(true);
});

test("E2E-009: menu and shortcut zoom share one persisted bounded value", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await desktop.app.evaluate(({ Menu }) => {
    const view = Menu.getApplicationMenu()?.items.find(item => item.label === "View");
    if (!view?.submenu) throw new Error("The View menu is missing.");
    const zoomIn = view.submenu.items.find(item => item.label === "Zoom In");
    if (!zoomIn) throw new Error("The Zoom In menu item is missing.");
    zoomIn.click();
  });
  await product.keyboard.press(`${process.platform === "darwin" ? "Meta" : "Control"}+-`);
  await product.keyboard.press(`${process.platform === "darwin" ? "Meta" : "Control"}+=`);
  const before = await desktop.app.evaluate(({ BrowserWindow }) =>
    BrowserWindow.getAllWindows()
      .find(window => {
        const url = window.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      })
      ?.webContents.getZoomLevel()
  );
  expect(before).toBe(1);
  await product.reload();
  await expect
    .poll(
      async () =>
        await desktop.app.evaluate(({ BrowserWindow }) =>
          BrowserWindow.getAllWindows()
            .find(window => {
              const url = window.webContents.getURL();
              return url.startsWith("http://") || url.startsWith("https://");
            })
            ?.webContents.getZoomLevel()
        )
    )
    .toBe(1);
  await desktop.closeShell();
  const relaunched = await launchDesktop({ home: desktop.home });
  await relaunched.product();
  expect(
    await relaunched.app.evaluate(({ BrowserWindow }) =>
      BrowserWindow.getAllWindows()
        .find(window => {
          const url = window.webContents.getURL();
          return url.startsWith("http://") || url.startsWith("https://");
        })
        ?.webContents.getZoomLevel()
    )
  ).toBe(1);
});

test("E2E-010: external navigation opens only safe web URLs and never leaves the product", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await desktop.app.evaluate(({ shell }) => {
    Reflect.set(globalThis, "__compozyExternalURLs", []);
    Reflect.set(shell, "openExternal", async (url: string) => {
      const urls = Reflect.get(globalThis, "__compozyExternalURLs") as string[];
      urls.push(url);
    });
  });
  await product.evaluate(() => {
    const safe = document.createElement("a");
    safe.href = "https://example.com/safe";
    safe.target = "_blank";
    document.body.append(safe);
    safe.click();
    window.open("file:///etc/passwd");
    window.location.href = "https://example.com/top-level";
  });
  await expect(product).toHaveURL(/^https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\]):\d+/u);
  expect(
    await desktop.app.evaluate(() => Reflect.get(globalThis, "__compozyExternalURLs"))
  ).toEqual(["https://example.com/safe", "https://example.com/top-level"]);
});

test("E2E-011: the daemon-served shell preserves the browser Settings journey and Chromium effects", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await completeOnboarding(product);
  await desktop.cli(["app", "open", "/settings/general"]);
  await expect(product).toHaveURL(/\/settings\/general(?:\?|$)/u);
  await expect(product.getByTestId("settings-shell")).toBeVisible();
  await expect(product.getByTestId("settings-section-nav")).toBeVisible();
  await expect(
    product.locator('[data-testid="settings-section-nav"] a[data-testid^="settings-section-"]')
  ).toHaveText([
    "General",
    "Appearance",
    "Layouts",
    "Providers",
    "Memory",
    "Roles",
    "Skills",
    "Automation",
    "Network",
    "Gateway",
    "Observability",
    "Hooks",
    "Extensions",
  ]);
  await expect(product.getByText("Updates", { exact: true })).toBeVisible();
  for (const section of ["network", "hooks", "extensions", "general"] as const) {
    await product.getByTestId(`settings-section-${section}`).click();
    await expect(product).toHaveURL(new RegExp(`/settings/${section}(?:\\?|$)`, "u"));
    await expect(product.getByTestId(`settings-page-${section}`)).toBeVisible();
  }
  expect(
    await product.evaluate(() => ({
      backdropFilter: CSS.supports("backdrop-filter", "blur(1px)"),
      chromium: navigator.userAgent.includes("Chrome/"),
    }))
  ).toEqual({ backdropFilter: true, chromium: true });
});

test("E2E-018: the real Settings API projects a journaled runtime swap through restart", async ({
  launchDesktop,
}) => {
  let coordinator = "";
  let replacement = "";
  let updateEnvironment: Readonly<Record<string, string>> = {};
  let updateHome = "";
  let updateProcess: RunningCommand | undefined;
  let updateResult:
    | Promise<{ readonly exitCode: number; readonly stderr: string; readonly stdout: string }>
    | undefined;
  const desktop = await launchDesktop({
    prepare: async context => {
      updateHome = context.home;
      updateEnvironment = context.environment;
      replacement = join(context.home, "bin", "compozy-next");
      coordinator = join(context.home, "runtime-update-fixture");
      await Promise.all([
        buildRuntimeFixture(replacement, "0.3.1", context.environment),
        buildRuntimeUpdateCoordinator(coordinator, context.environment),
        seedRuntimeUpdateCache(context.home),
      ]);
    },
  });
  const product = await desktop.product();
  await completeOnboarding(product);
  const child = startCommand(
    coordinator,
    ["--home", updateHome, "--replacement", replacement, "--phase-delay", "2s"],
    { env: updateEnvironment }
  );
  updateProcess = child;
  updateResult = child.result;
  await waitForUpdateOperation(updateHome);
  await product.goto(new URL("/settings/general", product.url()).toString(), {
    waitUntil: "domcontentloaded",
  });
  const liveStatus = await product.evaluate(async () => {
    const response = await fetch("/api/settings/update");
    if (!response.ok) throw new Error(`Update status failed with ${response.status}.`);
    return (await response.json()) as Record<string, unknown>;
  });
  expect(liveStatus.operation).toMatchObject({ targets: ["runtime"] });

  try {
    await desktop.cli(["app", "open", "/settings/general"]);
    const progress = product.getByTestId("settings-page-general-update-progress-runtime");
    for (const phase of ["download", "verify", "install", "start"]) {
      await expect(progress).toContainText(phase, { timeout: 30_000 });
    }
    if (!updateResult) throw new Error("The runtime update fixture did not start.");
    const result = await updateResult;
    expect(result.exitCode, result.stderr).toBe(0);
    expect(result.stdout.trim()).not.toBe("");
    await expect(product).toHaveTitle(/CompozyOS/u);
    await expect
      .poll(async () =>
        Reflect.get(await jsonCommand(desktop, ["version", "-o", "json"]), "Version")
      )
      .toBe("0.3.1");
    const historyRecords = (
      await readFile(join(desktop.home, "logs", "update-history.jsonl"), "utf8")
    )
      .split("\n")
      .filter(Boolean)
      .map(line => JSON.parse(line) as Record<string, unknown>);
    const terminal = historyRecords.at(-1);
    expect(terminal).toMatchObject({ outcome: "updated" });
    expect(terminal?.runtime).toMatchObject({ phase: "finalized", to_version: "0.3.1" });
  } finally {
    if (updateProcess && updateProcess.child.exitCode === null) {
      updateProcess.child.kill();
      await updateProcess.result;
    }
  }
});

test("E2E-012: renderer crashes reload within budget and surface the crash-loop dialog", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  let product = await desktop.product();
  const originalURL = product.url();
  await desktop.app.evaluate(() => {
    Reflect.set(globalThis, "__compozyCrashDialogs", 0);
    Reflect.set(globalThis, "__compozyCrashDecisions", []);
    Reflect.set(globalThis, "__compozyCrashDialogObserver", () => {
      const count = Reflect.get(globalThis, "__compozyCrashDialogs") as number;
      Reflect.set(globalThis, "__compozyCrashDialogs", count + 1);
    });
    Reflect.set(globalThis, "__compozyCrashDecisionObserver", (decision: string) => {
      const decisions = Reflect.get(globalThis, "__compozyCrashDecisions") as string[];
      decisions.push(decision);
    });
  });
  for (let index = 0; index < 4; index += 1) {
    await desktop.app.evaluate(({ BrowserWindow }) => {
      const window = BrowserWindow.getAllWindows().find(candidate => {
        const url = candidate.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      });
      if (!window) throw new Error("Product window missing before renderer crash.");
      window.webContents.forcefullyCrashRenderer();
    });
    await expect
      .poll(
        async () =>
          await desktop.app.evaluate(
            () => (Reflect.get(globalThis, "__compozyCrashDecisions") as string[]).length
          )
      )
      .toBe(index + 1);
    if (index < 3) {
      product = await desktop.product();
      await expect(product).toHaveURL(originalURL);
      await expect(product.getByTestId("os-desktop")).toBeVisible();
    }
  }
  expect(
    await desktop.app.evaluate(() => Reflect.get(globalThis, "__compozyCrashDecisions"))
  ).toEqual(["reload", "reload", "reload", "dialog"]);
  await expect
    .poll(
      async () => await desktop.app.evaluate(() => Reflect.get(globalThis, "__compozyCrashDialogs"))
    )
    .toBe(1);
  expect(
    ((await jsonCommand(desktop, ["status", "-o", "json"])).daemon as Record<string, unknown>)
      .status
  ).toBe("running");
});

test("E2E-013 E2E-014: running deep links navigate valid paths and collapse hostile payloads to home", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await desktop.spawnSecondary(["compozyos://open/sessions/e2e-session"]);
  await expect(product).toHaveURL(/\/sessions\/e2e-session(?:\?|$)/u);
  await desktop.spawnSecondary(["compozyos://open/route-that-does-not-exist"]);
  await expect(product.getByText("Route not found", { exact: true })).toBeVisible();
  for (const hostile of [
    "compozyos://open/http://evil.com",
    "compozyos://open/../../etc",
    "compozyos://open//host",
  ]) {
    await desktop.spawnSecondary([hostile]);
    await expect(product).toHaveURL(/^https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\]):\d+\/$/u);
  }
});

test("E2E-015 E2E-025: cold-start deep links and CLI paths preserve the validation boundary", async ({
  launchDesktop,
}) => {
  const cold = await launchDesktop({ argv: ["compozyos://open/settings/general"] });
  await expect(await cold.product()).toHaveURL(/\/settings\/general(?:\?|$)/u);
  await cold.closeShell();
  const hostile = await launchDesktop({ home: cold.home, argv: ["compozyos://open/../../etc"] });
  const product = await hostile.product();
  await expect(product).toHaveURL(/^https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\]):\d+\/$/u);
  await hostile.cli(["app", "open", "/tasks"]);
  await expect(product).toHaveURL(/\/tasks(?:\?|$)/u);
  for (const invalid of ["../etc", "/../etc", "//host", "/http://evil.com", "/bad\\path"]) {
    expect(await invalidAppOpenCode(hostile, invalid)).toBe("invalid_target_path");
  }
});

test("E2E-024: status, healthy retry, diagnose, and diagnostic bundle remain agent-manageable", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  await desktop.product();
  const recordPath = join(desktop.home, "app.json");
  await expect
    .poll(async () => Reflect.get(JSON.parse(await readFile(recordPath, "utf8")), "state"))
    .toBe("product");
  const before = JSON.parse(await readFile(recordPath, "utf8")) as Record<string, unknown>;
  expect(await jsonCommand(desktop, ["app", "retry", "-o", "json"])).toMatchObject({
    action: "retry",
    result: { ok: true },
  });
  const after = JSON.parse(await readFile(recordPath, "utf8")) as Record<string, unknown>;
  expect(after).toMatchObject({ pid: before.pid, state: "product" });
  const report = await jsonCommand(desktop, ["app", "diagnose", "-o", "json"]);
  expect(report).toMatchObject({ schema_version: 1, boot_phase: "product" });
  const bundle = await jsonCommand(desktop, ["app", "diagnose", "--bundle", "--yes", "-o", "json"]);
  const bundlePath = Reflect.get(bundle.bundle as Record<string, unknown>, "path");
  expect(typeof bundlePath).toBe("string");
  expect((await stat(String(bundlePath))).isFile()).toBe(true);
  const extracted = await mkdtemp(join(tmpdir(), "compozy-diagnostic-bundle-"));
  try {
    await extract({ cwd: extracted, file: String(bundlePath) });
    const manifest = JSON.parse(await readFile(join(extracted, "manifest.json"), "utf8")) as {
      kind?: unknown;
      report?: { boot_id?: unknown };
      schema_version?: unknown;
    };
    expect(manifest).toMatchObject({
      schema_version: 1,
      kind: "compozyos_desktop_diagnostics",
      report: { boot_id: report.boot_id },
    });
  } finally {
    await rm(extracted, { recursive: true, force: true });
  }
});

test("E2E-034: packaged product and boot windows enforce their security boundaries", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  expect(await product.evaluate(async () => await Notification.requestPermission())).toBe("denied");
  await desktop.app.evaluate(({ BrowserWindow }) => {
    const window = BrowserWindow.getAllWindows().find(candidate => {
      const url = candidate.webContents.getURL();
      return url.startsWith("http://") || url.startsWith("https://");
    });
    if (!window) throw new Error("The product window is missing.");
    window.webContents.openDevTools();
  });
  await expect
    .poll(
      async () =>
        await desktop.app.evaluate(({ BrowserWindow }) =>
          BrowserWindow.getAllWindows().some(window => window.webContents.isDevToolsOpened())
        )
    )
    .toBe(false);
  expect(
    await product.evaluate(() => ({
      nodeRequire: typeof Reflect.get(globalThis, "require"),
      nodeProcess: typeof Reflect.get(globalThis, "process"),
      webviewConstructor: document.createElement("webview").constructor.name,
      windowOpen: window.open("about:blank"),
    }))
  ).toEqual({
    nodeRequire: "undefined",
    nodeProcess: "undefined",
    webviewConstructor: "HTMLElement",
    windowOpen: null,
  });
  const productResponse = await product.request.get(product.url());
  const productCSP = productResponse.headers()["content-security-policy"] ?? "";
  for (const directive of [
    "connect-src 'self'",
    "worker-src 'self' blob:",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ]) {
    expect(productCSP).toContain(directive);
  }
  expect(productCSP).not.toContain("'unsafe-eval'");
  const cspProbe = await product.evaluate(async () => {
    const directives: string[] = [];
    const onViolation = (event: SecurityPolicyViolationEvent) => {
      directives.push(event.violatedDirective);
    };
    document.addEventListener("securitypolicyviolation", onViolation);
    Reflect.deleteProperty(globalThis, "__compozyInlineScriptRan");
    const script = document.createElement("script");
    script.textContent = "globalThis.__compozyInlineScriptRan = true";
    document.body.append(script);
    await fetch("https://example.com/compozy-csp-probe", { mode: "no-cors" }).catch(() => {});
    await new Promise(resolveDelay => setTimeout(resolveDelay, 50));
    document.removeEventListener("securitypolicyviolation", onViolation);
    return {
      directives,
      inlineScriptRan: Reflect.get(globalThis, "__compozyInlineScriptRan") === true,
    };
  });
  expect(cspProbe.inlineScriptRan).toBe(false);
  expect(cspProbe.directives.some(directive => directive.startsWith("script-src"))).toBe(true);
  expect(cspProbe.directives).toContain("connect-src");

  await desktop.closeShell();
  const productionHome = await mkdtemp(join(tmpdir(), "compozy-electron-security-"));
  const debugPort = await availableLoopbackPort();
  const production = startCommand(
    productionPackagedExecutable(),
    [`--remote-debugging-port=${debugPort}`],
    {
      env: {
        ...process.env,
        COMPOZY_DESKTOP_BUNDLE_ROOT: join(productionHome, "missing-runtime"),
        COMPOZY_DESKTOP_E2E: "1",
        COMPOZY_HOME: productionHome,
      },
    }
  );
  let productionProbeError: Error | null = null;
  const productionCleanupErrors: Error[] = [];
  try {
    await expect
      .poll(async () => {
        try {
          const state = JSON.parse(
            await readFile(join(productionHome, "app.json"), "utf8")
          ) as Record<string, unknown>;
          return state.pid;
        } catch (error) {
          if (error instanceof Error && "code" in error && error.code === "ENOENT") return 0;
          throw error;
        }
      })
      .toBe(production.child.pid);
    const probeDeadline = Date.now() + 1_000;
    while (Date.now() < probeDeadline) {
      try {
        await fetch(`http://127.0.0.1:${debugPort}/json/version`, {
          signal: AbortSignal.timeout(100),
        });
        throw new Error("The production desktop exposed a remote-debugging endpoint.");
      } catch (error) {
        if (!(error instanceof TypeError || error instanceof DOMException)) throw error;
      }
    }
  } catch (error) {
    productionProbeError = error instanceof Error ? error : new Error(String(error));
  } finally {
    if (production.child.exitCode === null && !production.child.kill("SIGTERM")) {
      productionCleanupErrors.push(
        new Error("The production security probe could not stop the process.")
      );
    }
    try {
      await production.result;
    } catch (error) {
      productionCleanupErrors.push(error instanceof Error ? error : new Error(String(error)));
    }
    try {
      await rm(productionHome, { recursive: true, force: true });
    } catch (error) {
      productionCleanupErrors.push(error instanceof Error ? error : new Error(String(error)));
    }
  }
  if (productionProbeError) {
    if (productionCleanupErrors.length > 0) {
      throw new AggregateError(
        [productionProbeError, ...productionCleanupErrors],
        "The production security probe and its cleanup failed."
      );
    }
    throw productionProbeError;
  }
  if (productionCleanupErrors.length === 1) throw productionCleanupErrors[0];
  if (productionCleanupErrors.length > 1) {
    throw new AggregateError(
      productionCleanupErrors,
      "The production security probe could not clean up."
    );
  }
  const failed = await launchDesktop({ prepare: corruptRuntimeBundle });
  const bootCSP = await failed.boot
    .locator('meta[http-equiv="Content-Security-Policy"]')
    .getAttribute("content");
  expect(bootCSP).toBe(
    "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; base-uri 'none'; form-action 'none'"
  );
  expect(
    await failed.boot.evaluate(async () => {
      Reflect.set(globalThis, "__compozyInlineExecuted", false);
      const script = document.createElement("script");
      script.textContent = "globalThis.__compozyInlineExecuted = true";
      document.head.append(script);
      await new Promise(resolveDelay => setTimeout(resolveDelay, 50));
      return Reflect.get(globalThis, "__compozyInlineExecuted");
    })
  ).toBe(false);
});
