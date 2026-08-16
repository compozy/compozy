import { cp, chmod, mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

import { expect, test, type DesktopInstance } from "../fixtures";

const repositoryRoot = resolve(import.meta.dir, "../../../..");

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
  const build = Bun.spawn(
    ["go", "build", "-trimpath", "-ldflags", ldflags, "-o", output, "./cmd/compozy"],
    { cwd: repositoryRoot, env: environment, stdout: "pipe", stderr: "pipe" }
  );
  const exitCode = await build.exited;
  if (exitCode !== 0) {
    throw new Error((await new Response(build.stderr).text()).trim() || "Fixture build failed.");
  }
  if (process.platform !== "win32") await chmod(output, 0o700);
}

async function startRuntimeFixture(runtime: string, environment: NodeJS.ProcessEnv): Promise<void> {
  const process = Bun.spawn([runtime, "daemon", "start"], {
    env: environment,
    stdout: "pipe",
    stderr: "pipe",
  });
  const exitCode = await process.exited;
  if (exitCode !== 0) {
    throw new Error((await new Response(process.stderr).text()).trim() || "Fixture start failed.");
  }
}

async function buildRuntimeUpdateCoordinator(
  output: string,
  environment: NodeJS.ProcessEnv
): Promise<void> {
  const build = Bun.spawn(
    ["go", "build", "-tags", "e2e", "-o", output, "./desktop/e2e/fixtures/runtimeupdate"],
    { cwd: repositoryRoot, env: environment, stdout: "pipe", stderr: "pipe" }
  );
  const exitCode = await build.exited;
  if (exitCode !== 0) {
    throw new Error(
      (await new Response(build.stderr).text()).trim() || "Update coordinator fixture build failed."
    );
  }
  if (process.platform !== "win32") await chmod(output, 0o700);
}

function availableUpdateSnapshot(): Record<string, unknown> {
  return {
    aggregate: "available",
    operation: null,
    runtime: {
      status: "available",
      install_method: "desktop-app",
      managed: false,
      current_version: "0.3.0",
      latest_version: "0.3.1",
      release_url: "https://example.test/releases/v0.3.1",
      daemon_restarted: false,
      message: "CompozyOS runtime 0.3.1 is available.",
    },
    app: {
      status: "up-to-date",
      running: true,
      current_version: "0.3.0",
      latest_version: "0.3.0",
      message: "CompozyOS app is up to date.",
    },
  };
}

async function runtimeUpdateSnapshot(
  home: string,
  started: boolean
): Promise<Record<string, unknown>> {
  if (!started) return availableUpdateSnapshot();
  let operation: Record<string, unknown> | null = null;
  try {
    operation = JSON.parse(await readFile(join(home, "update-operation.json"), "utf8")) as Record<
      string,
      unknown
    >;
  } catch (error) {
    if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) throw error;
  }
  if (!operation) {
    return {
      aggregate: "updated",
      operation: null,
      runtime: {
        status: "updated",
        install_method: "desktop-app",
        managed: false,
        current_version: "0.3.1",
        latest_version: "0.3.1",
        daemon_restarted: true,
        message: "CompozyOS runtime updated to 0.3.1.",
      },
      app: {
        status: "up-to-date",
        running: true,
        current_version: "0.3.0",
        latest_version: "0.3.0",
        message: "CompozyOS app is up to date.",
      },
    };
  }
  const runtime = Reflect.get(operation, "runtime") as Record<string, unknown>;
  return {
    aggregate: "applying",
    operation: {
      id: Reflect.get(operation, "operation_id"),
      revision: Reflect.get(operation, "revision"),
      targets: Reflect.get(operation, "targets"),
      active_target: Reflect.get(operation, "active_target"),
      phase: Reflect.get(runtime, "phase"),
      percent: Reflect.get(operation, "percent"),
      holder: Reflect.get(operation, "holder"),
      waiting: Reflect.get(operation, "waiting"),
      started_at: Reflect.get(operation, "started_at"),
    },
    runtime: {
      status: "applying",
      install_method: "desktop-app",
      managed: false,
      current_version: "0.3.0",
      latest_version: "0.3.1",
      daemon_restarted: false,
      message: "Updating CompozyOS runtime to 0.3.1.",
    },
    app: {
      status: "up-to-date",
      running: true,
      current_version: "0.3.0",
      latest_version: "0.3.0",
      message: "CompozyOS app is up to date.",
    },
  };
}

function isProductURL(url: string): boolean {
  return url.startsWith("http://") || url.startsWith("https://");
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
  expect(events.map(event => event.phase)).toEqual(
    expect.arrayContaining(["resolve", "provision", "start", "ready"])
  );
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
  copyPackagedExecutable,
  launchDesktop,
}) => {
  const packaged = await copyPackagedExecutable();
  await writeFile(packaged.bundleRuntimePath, "corrupted-runtime");
  const desktop = await launchDesktop({ executablePath: packaged.executablePath });
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
      await buildRuntimeFixture(runtime, "0.2.0", environment);
      await startRuntimeFixture(runtime, environment);
    },
  });
  await expect(
    desktop.boot.getByRole("heading", { name: "CompozyOS needs an update" })
  ).toBeVisible();
  await expect(desktop.boot.getByText(/Runtime 0\.2\.0.*Repair or update/u)).toBeVisible();
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
    desktop.spawnSecondary(["compozyos://open/settings"]),
  ]);
  await expect(product).toHaveURL(/\/settings(?:\?|$)/u);
  const windows = await desktop.app.evaluate(
    ({ BrowserWindow }) =>
      BrowserWindow.getAllWindows().filter(window => {
        const url = window.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      }).length
  );
  expect(windows).toBe(1);
  expect(
    await desktop.app.evaluate(({ BrowserWindow }) => BrowserWindow.getFocusedWindow() !== null)
  ).toBe(true);
});

test("E2E-005 E2E-007: relaunch attaches, stopped runtime starts once, and shell quit leaves it alive", async ({
  launchDesktop,
}) => {
  const first = await launchDesktop();
  await first.product();
  const initial = await jsonCommand(first, ["daemon", "status", "-o", "json"]);
  await first.closeShell();
  const alive = await jsonCommand(first, ["daemon", "status", "-o", "json"]);
  expect(alive).toMatchObject({ status: "running", pid: initial.pid });

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
  const restarted = await jsonCommand(started, ["daemon", "status", "-o", "json"]);
  expect(restarted.pid).not.toBe(initial.pid);
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
  await desktop.app.evaluate(
    async () => await new Promise(resolveDelay => setTimeout(resolveDelay, 400))
  );
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
    view?.submenu?.items.find(item => item.label === "Zoom In")?.click();
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
  await desktop.cli(["app", "open", "/settings"]);
  await expect(product).toHaveURL(/\/settings(?:\?|$)/u);
  await expect(product.getByText("Updates", { exact: true })).toBeVisible();
  expect(
    await product.evaluate(() => ({
      backdropFilter: CSS.supports("backdrop-filter", "blur(1px)"),
      chromium: navigator.userAgent.includes("Chrome/"),
    }))
  ).toEqual({ backdropFilter: true, chromium: true });
});

test("E2E-018: Settings drives a journaled runtime swap through restart and health verification", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  const replacement = join(desktop.home, "bin", "compozy-next");
  const coordinator = join(desktop.home, "runtime-update-fixture");
  await Promise.all([
    buildRuntimeFixture(replacement, "0.3.1", desktop.environment),
    buildRuntimeUpdateCoordinator(coordinator, desktop.environment),
  ]);

  let started = false;
  let updateProcess: ReturnType<typeof Bun.spawn> | undefined;
  let updateResult:
    | Promise<{ readonly exitCode: number; readonly stderr: string; readonly stdout: string }>
    | undefined;
  await product.route("**/api/settings/update*", async route => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    if (request.method() === "GET" && pathname === "/api/settings/update") {
      await route.fulfill({ json: await runtimeUpdateSnapshot(desktop.home, started) });
      return;
    }
    if (request.method() === "POST" && pathname === "/api/settings/update/apply") {
      started = true;
      const child = Bun.spawn(
        [coordinator, "--home", desktop.home, "--replacement", replacement, "--phase-delay", "2s"],
        { env: desktop.environment, stdout: "pipe", stderr: "pipe" }
      );
      updateProcess = child;
      updateResult = Promise.all([
        child.exited,
        new Response(child.stdout).text(),
        new Response(child.stderr).text(),
      ]).then(([exitCode, stdout, stderr]) => ({ exitCode, stdout, stderr }));
      await route.fulfill({
        json: {
          target: "runtime",
          status: "accepted",
          operation_id: "e2e-runtime-update",
          message: "Update accepted.",
          holder: null,
        },
      });
      return;
    }
    await route.continue();
  });

  try {
    await desktop.cli(["app", "open", "/settings"]);
    const apply = product.getByTestId("settings-page-general-update-apply-runtime");
    await expect(apply).toBeVisible();
    await apply.click();
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
    if (updateProcess?.exitCode === null) {
      updateProcess.kill();
      await updateProcess.exited;
    }
  }
});

test("E2E-012: renderer crashes reload within budget and surface the crash-loop dialog", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  let product = await desktop.product();
  const originalURL = product.url();
  await desktop.app.evaluate(({ dialog }) => {
    Reflect.set(globalThis, "__compozyCrashDialogs", 0);
    Reflect.set(dialog, "showMessageBox", async () => {
      const count = Reflect.get(globalThis, "__compozyCrashDialogs") as number;
      Reflect.set(globalThis, "__compozyCrashDialogs", count + 1);
      return { response: 0, checkboxChecked: false };
    });
  });
  for (let index = 0; index < 4; index += 1) {
    await desktop.app.evaluate(({ BrowserWindow }) => {
      const window = BrowserWindow.getAllWindows().find(candidate => {
        const url = candidate.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      });
      window?.webContents.forcefullyCrashRenderer();
    });
    if (index < 3) {
      await expect
        .poll(async () => desktop.app.windows().filter(page => isProductURL(page.url())).length)
        .toBe(1);
      product = desktop.app.windows().find(page => isProductURL(page.url())) ?? product;
      await expect(product).toHaveURL(originalURL);
    }
  }
  await expect
    .poll(
      async () => await desktop.app.evaluate(() => Reflect.get(globalThis, "__compozyCrashDialogs"))
    )
    .toBe(1);
  expect((await jsonCommand(desktop, ["daemon", "status", "-o", "json"])).status).toBe("running");
});

test("E2E-013 E2E-014: running deep links navigate valid paths and collapse hostile payloads to home", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await desktop.spawnSecondary(["compozyos://open/sessions/e2e-session"]);
  await expect(product).toHaveURL(/\/sessions\/e2e-session(?:\?|$)/u);
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
  const cold = await launchDesktop({ argv: ["compozyos://open/settings"] });
  await expect(await cold.product()).toHaveURL(/\/settings(?:\?|$)/u);
  await cold.closeShell();
  const hostile = await launchDesktop({ home: cold.home, argv: ["compozyos://open/../../etc"] });
  const product = await hostile.product();
  await expect(product).toHaveURL(/^https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\]):\d+\/$/u);
  await hostile.cli(["app", "open", "/tasks"]);
  await expect(product).toHaveURL(/\/tasks(?:\?|$)/u);
  for (const invalid of ["../etc", "/../etc", "//host", "/http://evil.com", "/bad\\path"]) {
    await expect(hostile.cli(["app", "open", invalid])).rejects.toThrow(
      /invalid_target_path|absolute product path|contains traversal/u
    );
  }
});

test("E2E-024: status, healthy retry, diagnose, and diagnostic bundle remain agent-manageable", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  await desktop.product();
  const recordPath = join(desktop.home, "app.json");
  const before = await readFile(recordPath, "utf8");
  expect(await jsonCommand(desktop, ["app", "retry", "-o", "json"])).toMatchObject({ ok: true });
  expect(await readFile(recordPath, "utf8")).toBe(before);
  const report = await jsonCommand(desktop, ["app", "diagnose", "-o", "json"]);
  expect(report).toMatchObject({ schema_version: 1, boot_phase: "product" });
  const bundle = await jsonCommand(desktop, ["app", "diagnose", "--bundle", "--yes", "-o", "json"]);
  const bundlePath = Reflect.get(bundle, "bundle_path");
  expect(typeof bundlePath).toBe("string");
  expect((await stat(String(bundlePath))).isFile()).toBe(true);
});

test("E2E-034: packaged product and boot windows enforce their security boundaries", async ({
  copyPackagedExecutable,
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  expect(await product.evaluate(async () => await Notification.requestPermission())).toBe("denied");
  await desktop.app.evaluate(({ BrowserWindow }) => {
    BrowserWindow.getAllWindows()
      .find(window => {
        const url = window.webContents.getURL();
        return url.startsWith("http://") || url.startsWith("https://");
      })
      ?.webContents.openDevTools();
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
  expect(productCSP).toContain("frame-ancestors 'none'");
  expect(productCSP).not.toContain("'unsafe-eval'");

  const packaged = await copyPackagedExecutable();
  await writeFile(packaged.bundleRuntimePath, "corrupted-runtime");
  const failed = await launchDesktop({ executablePath: packaged.executablePath });
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
