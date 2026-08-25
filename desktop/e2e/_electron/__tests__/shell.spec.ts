import { cp, chmod, mkdir, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { extract } from "tar";
import type { Page } from "@playwright/test";

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
const loginPathAgent = "desktop-login-path-agent";
const loginPathFixtureAgent = "browser-lifecycle-agent";
const loginPathPrompt = "e2e first message";

async function jsonCommand(
  desktop: DesktopInstance,
  arguments_: readonly string[]
): Promise<Record<string, unknown>> {
  return JSON.parse((await desktop.cli(arguments_)).stdout) as Record<string, unknown>;
}

async function firstWorkspaceID(desktop: DesktopInstance): Promise<string> {
  const workspaces: unknown = JSON.parse(
    (await desktop.cli(["workspace", "list", "-o", "json"])).stdout
  );
  if (!Array.isArray(workspaces)) throw new Error("Workspace list did not return an array.");
  const first = workspaces[0];
  if (!first || typeof first !== "object") throw new Error("Workspace list is empty.");
  const id = Reflect.get(first, "id");
  if (typeof id !== "string" || id.trim() === "") {
    throw new Error("The first workspace has no stable id.");
  }
  return id;
}

async function waitForGlobalShortcut(
  product: Page,
  commandID: string,
  status: "registered" | "failed_in_use"
): Promise<Record<string, unknown>> {
  let observed: Record<string, unknown> | undefined;
  await expect
    .poll(async () => {
      observed = await product.evaluate(async id => {
        const bridge = Reflect.get(window, "compozyShell") as
          | {
              globalShortcuts: {
                status(): Promise<
                  Array<{
                    command_id: string;
                    intended_chord: string;
                    active_chord?: string;
                    status: string;
                  }>
                >;
              };
            }
          | undefined;
        if (!bridge) return undefined;
        return (await bridge.globalShortcuts.status()).find(item => item.command_id === id);
      }, commandID);
      return observed?.status;
    })
    .toBe(status);
  if (!observed) throw new Error(`Global shortcut ${commandID} did not report status ${status}.`);
  return observed;
}

async function invokeGlobalShortcut(desktop: DesktopInstance, accelerator: string): Promise<void> {
  const invoked = await desktop.app.evaluate((_electron, value) => {
    const invoke = Reflect.get(globalThis, "__compozyGlobalShortcutInvoke");
    if (typeof invoke !== "function") return false;
    return invoke(value) === true;
  }, accelerator);
  if (!invoked) throw new Error(`Global shortcut ${accelerator} is not registered.`);
}

async function copyPaletteExtensionFixture(home: string): Promise<string> {
  const source = join(home, "fixtures", "palette-fixture-go");
  await cp(
    join(repositoryRoot, "internal", "extension", "testdata", "palette-fixture-go"),
    source,
    {
      recursive: true,
    }
  );
  const modulePath = join(source, "go.mod");
  const moduleFile = await readFile(modulePath, "utf8");
  await writeFile(
    modulePath,
    `${moduleFile.trimEnd()}\n\nreplace github.com/compozy/compozy/sdk/go => ${join(repositoryRoot, "sdk", "go")}\n`,
    { mode: 0o600 }
  );
  return source;
}

async function bootstrapEvents(home: string): Promise<readonly Record<string, unknown>[]> {
  const contents = await readFile(join(home, "logs", "desktop-bootstrap.jsonl"), "utf8");
  return contents
    .split("\n")
    .filter(Boolean)
    .map(line => JSON.parse(line) as Record<string, unknown>);
}

async function prepareLoginPathProvider(context: {
  readonly environment: Record<string, string>;
  readonly home: string;
}): Promise<void> {
  const operatorHome = join(context.home, "operator");
  const loginBin = join(operatorHome, ".local", "bin");
  const fixtureDir = join(context.home, "fixtures");
  const driver = join(
    fixtureDir,
    process.platform === "win32" ? "acpmock-driver.exe" : "acpmock-driver"
  );
  const provider = join(loginBin, process.platform === "win32" ? "desktop-acp.cmd" : "desktop-acp");
  const fixture = join(
    repositoryRoot,
    "internal",
    "testutil",
    "acpmock",
    "testdata",
    "browser_session_lifecycle_fixture.json"
  );
  const diagnostics = join(context.home, "logs", "desktop-login-path-provider.jsonl");
  const agentDir = join(context.home, "agents", loginPathAgent);
  await Promise.all([
    mkdir(loginBin, { recursive: true, mode: 0o700 }),
    mkdir(fixtureDir, { recursive: true, mode: 0o700 }),
    mkdir(agentDir, { recursive: true, mode: 0o700 }),
  ]);
  const build = await runCommand(
    "go",
    ["build", "-o", driver, "./internal/testutil/acpmock/cmd/acpmock-driver"],
    { cwd: repositoryRoot, env: process.env }
  );
  if (build.exitCode !== 0) {
    throw new Error(build.stderr.trim() || "ACP provider fixture build failed.");
  }
  if (process.platform !== "win32") await chmod(driver, 0o700);

  context.environment.COMPOZY_DESKTOP_TEST_DRIVER = driver;
  context.environment.COMPOZY_DESKTOP_TEST_FIXTURE = fixture;
  context.environment.COMPOZY_DESKTOP_TEST_DIAGNOSTICS = diagnostics;
  context.environment.COMPOZY_DESKTOP_TEST_LOGIN_BIN = loginBin;
  context.environment.HOME = operatorHome;
  context.environment.PATH =
    process.platform === "win32" ? (process.env.PATH ?? "") : "/usr/bin:/bin:/usr/sbin:/sbin";

  if (process.platform === "win32") {
    await writeFile(
      provider,
      [
        "@echo off",
        '"%COMPOZY_DESKTOP_TEST_DRIVER%" --fixture "%COMPOZY_DESKTOP_TEST_FIXTURE%" --agent "browser-lifecycle-agent" --diagnostics "%COMPOZY_DESKTOP_TEST_DIAGNOSTICS%"',
        "",
      ].join("\r\n"),
      { mode: 0o700 }
    );
  } else {
    const loginShell = join(operatorHome, "login-shell");
    context.environment.SHELL = loginShell;
    await Promise.all([
      writeFile(
        provider,
        `#!/bin/sh\nexec "$COMPOZY_DESKTOP_TEST_DRIVER" --fixture "$COMPOZY_DESKTOP_TEST_FIXTURE" --agent "${loginPathFixtureAgent}" --diagnostics "$COMPOZY_DESKTOP_TEST_DIAGNOSTICS"\n`,
        { mode: 0o700 }
      ),
      writeFile(
        loginShell,
        '#!/bin/sh\nexport PATH="$COMPOZY_DESKTOP_TEST_LOGIN_BIN:/usr/bin:/bin:/usr/sbin:/sbin"\nexec /bin/sh "$@"\n',
        { mode: 0o700 }
      ),
    ]);
  }

  const configPath = join(context.home, "config.toml");
  const config = await readFile(configPath, "utf8");
  await Promise.all([
    writeFile(
      configPath,
      `${config}\n[providers.desktop-login-path]\ncommand = "desktop-acp"\ndisplay_name = "Desktop login PATH"\nauth_mode = "none"\nnone_security = "local_transport"\n\n[providers.desktop-login-path.models]\ndefault = "qa-browser-model"\n`
    ),
    writeFile(
      join(agentDir, "AGENT.md"),
      `---\nname: ${loginPathAgent}\nprovider: desktop-login-path\nmodel: qa-browser-model\npermissions: approve-reads\n---\n\nYou are the desktop login PATH test agent.\n`,
      { mode: 0o600 }
    ),
  ]);
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
  expect(
    await desktop.app.evaluate(
      ({ BrowserWindow }) =>
        BrowserWindow.getAllWindows().filter(window => window.isFocused()).length
    )
  ).toBe(0);
  const product = await desktop.product();
  expect(
    await desktop.app.evaluate(
      ({ BrowserWindow }) =>
        BrowserWindow.getAllWindows().filter(window => window.isFocused()).length
    )
  ).toBe(0);
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

test("E2E-008: native product chrome and window geometry survive relaunch", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  if (process.platform === "darwin" || process.platform === "linux") {
    const overlay = await product.evaluate(() => {
      const windowControlsOverlay = Reflect.get(navigator, "windowControlsOverlay") as
        | {
            getTitlebarAreaRect(): DOMRect;
            visible: boolean;
          }
        | undefined;
      if (!windowControlsOverlay) return null;
      const area = windowControlsOverlay.getTitlebarAreaRect();
      return {
        height: area.height,
        narrowerThanWindow: area.width < window.innerWidth,
        visible: windowControlsOverlay.visible,
      };
    });
    expect(overlay).toEqual({
      height: 44,
      narrowerThanWindow: true,
      visible: true,
    });
  }
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

test("native Edit shortcuts copy and paste editable renderer content", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await completeOnboarding(product);
  expect(
    await desktop.app.evaluate(({ Menu }) => {
      const edit = Menu.getApplicationMenu()?.items.find(item => item.label === "Edit");
      return edit?.submenu?.items.map(item => item.role) ?? [];
    })
  ).toEqual(expect.arrayContaining(["cut", "copy", "paste", "selectall"]));
  await product.evaluate(() => {
    const source = document.createElement("input");
    source.setAttribute("aria-label", "Copy source");
    source.value = "Copied through the Electron Edit menu";
    const target = document.createElement("input");
    target.setAttribute("aria-label", "Paste target");
    document.body.append(source, target);
    source.focus();
    source.select();
  });

  const modifier = process.platform === "darwin" ? "Meta" : "Control";
  await product.keyboard.press(`${modifier}+C`);
  await product.getByRole("textbox", { name: "Paste target" }).focus();
  await product.keyboard.press(`${modifier}+V`);

  await expect(product.getByRole("textbox", { name: "Paste target" })).toHaveValue(
    "Copied through the Electron Edit menu"
  );
});

test("desktop-owned runtime resolves a provider available only on the login PATH", async ({
  launchDesktop,
}) => {
  test.skip(process.platform === "win32", "Windows preserves the inherited desktop PATH");

  const desktop = await launchDesktop({ prepare: prepareLoginPathProvider });
  await desktop.product();
  const created = await jsonCommand(desktop, [
    "session",
    "new",
    "--cwd",
    repositoryRoot,
    "--agent",
    loginPathAgent,
    "-o",
    "json",
  ]);
  const sessionID = typeof created.id === "string" ? created.id : "";
  expect(sessionID).not.toBe("");

  await desktop.cli(["session", "prompt", sessionID, loginPathPrompt, "-o", "json"]);
  await expect
    .poll(async () => {
      try {
        return await readFile(
          join(desktop.home, "logs", "desktop-login-path-provider.jsonl"),
          "utf8"
        );
      } catch (error) {
        if (error instanceof Error && "code" in error && error.code === "ENOENT") return "";
        throw error;
      }
    })
    .toContain(loginPathPrompt);
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
    "Attention",
    "Observability",
    "Gateway",
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
  await expect(product.getByRole("heading", { name: "Page not found" })).toBeVisible();
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
  const coldProduct = await cold.product();
  await expect(coldProduct).toHaveURL(/\/settings\/general(?:\?|$)/u);
  await completeOnboarding(coldProduct);
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

test("E2E-034: packaged windows enforce security boundaries and intentional debugging", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop({
    environment: { COMPOZY_DESKTOP_E2E_FOREGROUND: "1" },
  });
  const product = await desktop.product();
  expect(await product.evaluate(async () => await Notification.requestPermission())).toBe("denied");

  const clipboardValue = "Copied through the packaged Electron product window";
  expect(
    await product.evaluate(async value => {
      await navigator.clipboard.writeText(value);
      return true;
    }, clipboardValue)
  ).toBe(true);
  expect(await desktop.app.evaluate(({ clipboard }) => clipboard.readText())).toBe(clipboardValue);

  // Playwright's page keyboard does not enter Electron's native input path on macOS;
  // sendInputEvent supplies the exact Control+Option+I chord that a physical keypress emits.
  await desktop.app.evaluate(({ BrowserWindow }) => {
    const productWindow = BrowserWindow.getAllWindows().find(window =>
      /^https?:/u.test(window.webContents.getURL())
    );
    if (!productWindow) throw new Error("Product window missing before shortcut probe.");
    productWindow.webContents.sendInputEvent({
      type: "keyDown",
      keyCode: "I",
      modifiers: ["control", "alt"],
    });
    productWindow.webContents.sendInputEvent({
      type: "keyUp",
      keyCode: "I",
      modifiers: ["control", "alt"],
    });
  });
  await expect
    .poll(
      async () =>
        await desktop.app.evaluate(({ BrowserWindow }) =>
          BrowserWindow.getAllWindows().some(window => window.webContents.isDevToolsOpened())
        )
    )
    .toBe(true);
  await desktop.app.evaluate(({ BrowserWindow }) => {
    const productWindow = BrowserWindow.getAllWindows().find(window =>
      /^https?:/u.test(window.webContents.getURL())
    );
    if (!productWindow) throw new Error("Product window missing before shortcut close.");
    productWindow.webContents.sendInputEvent({
      type: "keyDown",
      keyCode: "I",
      modifiers: ["control", "alt"],
    });
    productWindow.webContents.sendInputEvent({
      type: "keyUp",
      keyCode: "I",
      modifiers: ["control", "alt"],
    });
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

test("E2E-027: a global summon restores and focuses the palette without crossing an open modal", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop({
    environment: { COMPOZY_DESKTOP_E2E_FOREGROUND: "1" },
  });
  const product = await desktop.product();
  await completeOnboarding(product);
  await waitForGlobalShortcut(product, "palette.summon.global", "registered");

  await desktop.app.evaluate(async ({ BrowserWindow }) => {
    const productWindow = BrowserWindow.getAllWindows().find(window =>
      /^https?:/u.test(window.webContents.getURL())
    );
    if (!productWindow) throw new Error("Product window missing before global summon.");
    productWindow.minimize();
    const other = new BrowserWindow({ width: 320, height: 240, show: true });
    other.setTitle("Shortcut focus fixture");
    await other.loadURL("about:blank");
    other.focus();
  });
  expect(
    await desktop.app.evaluate(({ BrowserWindow }) => {
      const productWindow = BrowserWindow.getAllWindows().find(window =>
        /^https?:/u.test(window.webContents.getURL())
      );
      return productWindow?.isMinimized() ?? false;
    })
  ).toBe(true);

  await invokeGlobalShortcut(desktop, "CommandOrControl+Shift+Space");
  await expect(product.getByRole("dialog", { name: "Command palette" })).toBeVisible();
  await expect
    .poll(
      async () =>
        await desktop.app.evaluate(({ BrowserWindow }) => {
          const productWindow = BrowserWindow.getAllWindows().find(window =>
            /^https?:/u.test(window.webContents.getURL())
          );
          return productWindow
            ? { focused: productWindow.isFocused(), minimized: productWindow.isMinimized() }
            : null;
        })
    )
    .toEqual({ focused: true, minimized: false });
  await product.keyboard.press("Escape");

  await product.getByRole("button", { name: "New session" }).last().click();
  const modal = product.getByRole("dialog").first();
  await expect(modal).toBeVisible();
  await invokeGlobalShortcut(desktop, "CommandOrControl+Shift+Space");
  await expect(product.getByRole("dialog", { name: "Command palette" })).toHaveCount(0);
  await expect
    .poll(async () => await modal.evaluate(node => node.contains(document.activeElement)))
    .toBe(true);

  await desktop.app.evaluate(({ BrowserWindow }) => {
    BrowserWindow.getAllWindows()
      .filter(window => window.getTitle() === "Shortcut focus fixture")
      .forEach(window => window.destroy());
  });
});

test("E2E-028: an argument command global hotkey summons directly into argument mode", async ({
  launchDesktop,
}) => {
  let extensionSource = "";
  const desktop = await launchDesktop({
    environment: { COMPOZY_DESKTOP_E2E_FOREGROUND: "1" },
    prepare: async context => {
      extensionSource = await copyPaletteExtensionFixture(context.home);
    },
  });
  const product = await desktop.product();
  await completeOnboarding(product);
  const workspaceID = await firstWorkspaceID(desktop);
  await desktop.cli([
    "extension",
    "dev",
    extensionSource,
    "--workspace",
    workspaceID,
    "-o",
    "json",
  ]);
  await desktop.cli([
    "cmd-palette",
    "bind",
    "ext.notes.capture",
    "meta+alt+KeyN",
    "--global",
    "--workspace",
    workspaceID,
    "-o",
    "json",
  ]);
  await waitForGlobalShortcut(product, "ext.notes.capture", "registered");

  await product.keyboard.press(process.platform === "darwin" ? "Meta+K" : "Control+K");
  await product.getByRole("combobox").fill("capture note");
  await expect(product.getByTestId("os-palette-command-ext.notes.capture")).toBeVisible();
  await product.keyboard.press("Escape");

  await desktop.app.evaluate(async ({ BrowserWindow }) => {
    const other = new BrowserWindow({ width: 320, height: 240, show: true });
    other.setTitle("Argument shortcut focus fixture");
    await other.loadURL("about:blank");
    other.focus();
  });
  await invokeGlobalShortcut(desktop, "CommandOrControl+Alt+N");
  await expect(product.getByTestId("os-palette-args")).toBeVisible();
  await expect(product.getByTestId("os-palette-arg-title")).toBeFocused();
  await desktop.app.evaluate(({ BrowserWindow }) => {
    BrowserWindow.getAllWindows()
      .filter(window => window.getTitle() === "Argument shortcut focus fixture")
      .forEach(window => window.destroy());
  });
});

test("E2E-029: a captured replacement restores the previous binding and relaunch reports fresh truth", async ({
  launchDesktop,
}) => {
  const desktop = await launchDesktop({
    environment: { COMPOZY_DESKTOP_E2E_FOREGROUND: "1" },
  });
  const product = await desktop.product();
  await completeOnboarding(product);
  const workspaceID = await firstWorkspaceID(desktop);
  await waitForGlobalShortcut(product, "palette.summon.global", "registered");
  expect(
    await desktop.app.evaluate(({ globalShortcut }) =>
      globalShortcut.register("CommandOrControl+Alt+Space", () => undefined)
    )
  ).toBe(true);

  await desktop.cli([
    "cmd-palette",
    "bind",
    "palette.summon.global",
    "meta+alt+Space",
    "--global",
    "--workspace",
    workspaceID,
    "-o",
    "json",
  ]);
  const failed = await waitForGlobalShortcut(product, "palette.summon.global", "failed_in_use");
  expect(failed).toMatchObject({
    active_chord: "meta+shift+Space",
    intended_chord: "meta+alt+Space",
  });
  await product.goto(new URL("/settings/layouts", product.url()).toString());
  const globalSection = product.getByTestId("window-manager-global-hotkeys");
  await expect(globalSection).toContainText("unavailable — in use by another application");
  await expect(globalSection).toContainText("captured");

  await invokeGlobalShortcut(desktop, "CommandOrControl+Shift+Space");
  await expect(product.getByRole("dialog", { name: "Command palette" })).toBeVisible();
  await desktop.closeShell();

  const relaunched = await launchDesktop({
    home: desktop.home,
    environment: { COMPOZY_DESKTOP_E2E_FOREGROUND: "1" },
  });
  const relaunchedProduct = await relaunched.product();
  const registered = await waitForGlobalShortcut(
    relaunchedProduct,
    "palette.summon.global",
    "registered"
  );
  expect(registered).toMatchObject({
    active_chord: "meta+alt+Space",
    intended_chord: "meta+alt+Space",
  });
  await relaunchedProduct.goto(new URL("/settings/layouts", relaunchedProduct.url()).toString());
  await expect(relaunchedProduct.getByTestId("window-manager-global-hotkeys")).toContainText(
    "active"
  );
});

test("E2E-030: a plain browser explains global hotkeys while keeping the in-app palette chord", async ({
  launchDesktop,
  playwright,
}) => {
  const desktop = await launchDesktop();
  const product = await desktop.product();
  await completeOnboarding(product);
  const browser = await playwright.chromium.launch();
  try {
    const page = await browser.newPage();
    await page.goto(new URL("/settings/layouts", product.url()).toString());
    const globalSection = page.getByTestId("window-manager-global-hotkeys");
    await expect(globalSection).toContainText("desktop only");
    await expect(globalSection).toContainText("requires desktop shell");
    await expect(
      globalSection.getByRole("button", { name: /Record global hotkey/u }).first()
    ).toBeDisabled();

    await page.keyboard.press(process.platform === "darwin" ? "Meta+K" : "Control+K");
    await expect(page.getByRole("dialog", { name: "Command palette" })).toBeVisible();
  } finally {
    await browser.close();
  }
});
