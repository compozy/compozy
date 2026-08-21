import { randomUUID } from "node:crypto";
import { access, mkdir } from "node:fs/promises";
import { performance } from "node:perf_hooks";
import { join, resolve } from "node:path";

import { app, globalShortcut, ipcMain, session, shell, systemPreferences } from "electron";

import { BootstrapRunner } from "./bootstrap/bootstrap-runner";
import { RuntimeHealthMonitor } from "./bootstrap/runtime-health-monitor";
import { bootstrapSnapshot } from "./boot/bootstrap-state";
import { isForwardedBootMethod } from "./boot/boot-contract";
import { createControlHandler } from "./control/control-handler";
import { ControlMethodError } from "./control/control-contract";
import { ControlServer } from "./control/control-server";
import { deleteControlToken, rotateControlToken } from "./control/control-token";
import {
  desktopProtocolName,
  parseDeepLink,
  productDeepLink,
  DeepLinkQueue,
  lastDeepLink,
} from "./deep-links/deep-link";
import { exportDiagnostics } from "./diagnostics/export-diagnostics";
import { MINIMUM_RUNTIME } from "./generated/build-config";
import { resolveDesktopPaths, runtimeExecutableName } from "./home";
import { DesktopLogger } from "./logging/desktop-logger";
import { AppStatePublisher } from "./state/app-state";
import { publicSafeText } from "./state/public-safe-text";
import { ProductBridgeController } from "./product/product-bridge-controller";
import { detectAccessibility } from "./shortcuts/accessibility";
import { ElectronGlobalShortcut } from "./shortcuts/electron-global-shortcut";
import { GlobalShortcutPolicy } from "./shortcuts/global-shortcut-policy";
import { AppUpdateConsumer } from "./update/app-update-consumer";
import { ElectronUpdateInstaller } from "./update/electron-installer";
import { OperationWatcher } from "./update/operation-watcher";
import { UpdateTransitionClient } from "./update/transition-client";
import { BootWindow } from "./window/boot-window";
import { ProductWindow } from "./window/product-window";
import type { WindowPresentation } from "./window/window-presentation";
import { applyDefaultDenyPermissions } from "./window/security";
import { installApplicationMenu } from "./window/application-menu";

declare const __COMPOZY_DESKTOP_E2E_BUILD__: boolean;
declare const __COMPOZY_RELEASE_CHANNEL__: "beta" | "development";

const paths = resolveDesktopPaths();
const bootId = randomUUID();
const processStartedAt = new Date(performance.timeOrigin);
const links = new DeepLinkQueue();
const logger = new DesktopLogger(paths.desktopLog, bootId);
const windowPresentation: WindowPresentation =
  process.env.COMPOZY_DESKTOP_E2E === "1" && process.env.COMPOZY_DESKTOP_E2E_FOREGROUND !== "1"
    ? "inactive"
    : "foreground";
let product: ProductWindow | null = null;
let boot: BootWindow | null = null;
let controlServer: ControlServer | null = null;
let operationWatcher: OperationWatcher | null = null;
let updateConsumer: AppUpdateConsumer | null = null;
let runtimeMonitor: RuntimeHealthMonitor | null = null;
let publisher: AppStatePublisher | null = null;
let productBridge: ProductBridgeController | null = null;
let cleanupPromise: Promise<void> | null = null;
let cleanupComplete = false;

async function waitForE2EProductReady(): Promise<void> {
  const readyPath = process.env.COMPOZY_DESKTOP_E2E_READY_FILE?.trim();
  if (process.env.COMPOZY_DESKTOP_E2E !== "1" || !readyPath) return;
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    try {
      await access(readyPath);
      return;
    } catch (error) {
      if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) throw error;
    }
    await new Promise(resolveDelay => setTimeout(resolveDelay, 25));
  }
  throw new Error("The desktop E2E product-ready gate timed out.");
}

if (app.isPackaged && !__COMPOZY_DESKTOP_E2E_BUILD__) {
  for (const name of ["remote-debugging-port", "remote-debugging-pipe", "inspect", "inspect-brk"]) {
    app.commandLine.removeSwitch(name);
  }
}

app.setPath("userData", join(paths.home, "electron"));
app.on("open-url", (event, url) => {
  event.preventDefault();
  links.push(url);
  product?.focus();
});

function resourcePaths(): {
  bundle: string;
  manifest: string;
  page: string;
  bootPreload: string;
  productPreload: string;
} {
  const packagedRoot = process.resourcesPath;
  const bundleRoot = process.env.COMPOZY_DESKTOP_BUNDLE_ROOT?.trim()
    ? resolve(process.env.COMPOZY_DESKTOP_BUNDLE_ROOT)
    : join(packagedRoot, "runtime");
  return {
    bundle: join(bundleRoot, runtimeExecutableName()),
    manifest: join(bundleRoot, "runtime-manifest.json"),
    page: app.isPackaged
      ? join(packagedRoot, "pages", "boot.html")
      : join(__dirname, "pages", "boot.html"),
    bootPreload: join(__dirname, "boot-preload.cjs"),
    productPreload: join(__dirname, "product-preload.cjs"),
  };
}

async function cleanup(): Promise<void> {
  if (cleanupPromise) return await cleanupPromise;
  cleanupPromise = (async () => {
    const errors: Error[] = [];
    const attempt = async (step: () => void | Promise<void>): Promise<void> => {
      try {
        await step();
      } catch (error) {
        errors.push(error instanceof Error ? error : new Error(String(error)));
      }
    };
    try {
      await attempt(async () => await product?.flushState());
      await attempt(() => productBridge?.unregister());
      await attempt(() => operationWatcher?.stop());
      await attempt(() => updateConsumer?.stop());
      await attempt(() => runtimeMonitor?.stop());
      await attempt(async () => {
        if (controlServer) await controlServer.close();
      });
      await attempt(async () => await deleteControlToken(paths.appToken));
      await attempt(async () => await publisher?.markCleanShutdown());
      await attempt(async () => await logger.flush());
      if (errors.length === 1) throw errors[0];
      if (errors.length > 1) throw new AggregateError(errors, "Desktop cleanup failed.");
    } finally {
      cleanupComplete = true;
    }
  })();
  return await cleanupPromise;
}

async function quitCleanly(): Promise<void> {
  try {
    await cleanup();
  } catch (error) {
    logger.error("clean desktop shutdown", error);
  }
  app.quit();
}

async function start(): Promise<void> {
  await mkdir(paths.home, { recursive: true, mode: 0o700 });
  const resources = resourcePaths();
  const shortcutRuntime = new ElectronGlobalShortcut(globalShortcut);
  const shortcutPolicy = new GlobalShortcutPolicy({
    globalShortcut: shortcutRuntime,
    accessibility: () =>
      __COMPOZY_DESKTOP_E2E_BUILD__
        ? { allowed: true }
        : detectAccessibility({
            platform: process.platform,
            isTrusted: () => systemPreferences.isTrustedAccessibilityClient(false),
          }),
    onInvoke: commandID => {
      product?.focus();
      product?.send("shell:summon", { command_id: commandID });
    },
  });
  productBridge = new ProductBridgeController({ ipcMain, shortcuts: shortcutPolicy });
  productBridge.register();
  if (__COMPOZY_DESKTOP_E2E_BUILD__) {
    Reflect.set(globalThis, "__compozyGlobalShortcutInvoke", (accelerator: string) =>
      shortcutRuntime.invokeForE2E(accelerator)
    );
  }
  const statePublisher = await AppStatePublisher.create({
    path: paths.appRecord,
    appVersion: app.getVersion(),
    channel: __COMPOZY_RELEASE_CHANNEL__,
    startedAt: processStartedAt,
    bootId,
    startupMarker: paths.startupMarker,
    onPublished: snapshot => boot?.render(snapshot),
  });
  publisher = statePublisher;
  boot = new BootWindow({
    pagePath: resources.page,
    preloadPath: resources.bootPreload,
    presentation: windowPresentation,
    onError: error => logger.error("load boot window", error),
  });
  installApplicationMenu(() => product);
  await statePublisher.publish({ state: "resolving" });

  const controlHandler = createControlHandler(statePublisher, {
    navigate: async path => {
      const target = parseDeepLink(productDeepLink(path));
      if (target.kind !== "product") {
        throw new ControlMethodError("invalid_target_path", "The product path is invalid.");
      }
      links.push(productDeepLink(target.path));
      product?.focus();
      boot?.show();
      return target.path;
    },
    retry: async () => await runBootstrap(),
    exportDiagnostics: async () =>
      await exportDiagnostics({
        home: paths.home,
        logs: paths.logs,
        report: statePublisher.diagnosticReport(),
      }),
  });
  const token = await rotateControlToken(paths.appToken);
  controlServer = await ControlServer.start(paths.appSocket, token, controlHandler, error => {
    logger.error("serve app control request", error);
  });
  ipcMain.handle("boot:control", async (_event, method: unknown, params: unknown) => {
    if (method === "open_logs") {
      const failure = await shell.openPath(paths.logs);
      if (failure) throw new Error(failure);
      return { opened: true };
    }
    if (method === "quit") {
      void quitCleanly();
      return { quitting: true };
    }
    if (typeof method !== "string") throw new Error("The boot action is invalid.");
    if (isForwardedBootMethod(method)) {
      return await controlHandler(method, params);
    }
    throw new Error("The boot action is not supported.");
  });

  const runner = new BootstrapRunner({
    bundlePath: resources.bundle,
    manifestPath: resources.manifest,
    logPath: paths.bootstrapLog,
    minimumRuntime: MINIMUM_RUNTIME,
    appVersion: app.getVersion(),
    channel: __COMPOZY_RELEASE_CHANNEL__,
    bootId,
  });
  let bootstrapFlow: Promise<void> | null = null;
  async function runBootstrap(): Promise<void> {
    if (bootstrapFlow) return await bootstrapFlow;
    bootstrapFlow = runBootstrapAttempt();
    try {
      await bootstrapFlow;
    } finally {
      bootstrapFlow = null;
    }
  }

  async function runBootstrapAttempt(): Promise<void> {
    try {
      const runtime = await runner.run(async event => {
        await statePublisher.publish(bootstrapSnapshot(event, paths.bootstrapLog));
      });
      await statePublisher.setRuntime(runtime.version, runtime.owned);
      startUpdateConsumer(statePublisher);
      runtimeMonitor?.stop();
      product = new ProductWindow({
        origin: runtime.origin,
        windowStatePath: paths.windowState,
        preloadPath: resources.productPreload,
        presentation: windowPresentation,
        links,
        onReady: async () => {
          await statePublisher.publish({
            state: "product",
            origin: runtime.origin,
            owned: runtime.owned,
          });
          runtimeMonitor = new RuntimeHealthMonitor({
            origin: runtime.origin,
            onDisconnected: async () => {
              await product?.close();
              product = null;
              await statePublisher.publish({ state: "disconnected" });
              boot?.show();
            },
          });
          runtimeMonitor.start();
          boot?.close();
        },
        onLoadFailure: async error => {
          await statePublisher.publish({
            state: "error",
            error: {
              code: "load_deadline_exceeded",
              safe_message: publicSafeText(error.message, "The product window did not load."),
              log_path: paths.desktopLog,
            },
          });
          boot?.show();
        },
        onError: error => logger.error("product window", error),
      });
      await waitForE2EProductReady();
      await product.create();
    } catch (error) {
      logger.error("bootstrap runtime", error);
      const terminal = statePublisher.snapshot();
      if (terminal.state !== "error" && terminal.state !== "skew") {
        await statePublisher.publish({
          state: "error",
          error: {
            code: "boot_window_failed",
            safe_message: publicSafeText(
              error instanceof Error ? error.message : error,
              "CompozyOS could not start."
            ),
            log_path: paths.bootstrapLog,
          },
        });
      }
      boot?.show();
    }
  }

  function startUpdateConsumer(state: AppStatePublisher): void {
    if (operationWatcher) return;
    updateConsumer = new AppUpdateConsumer({
      currentVersion: app.getVersion(),
      transitions: new UpdateTransitionClient(resources.bundle),
      installer: new ElectronUpdateInstaller(),
      onError: error => logger.error("consume app update operation", error),
    });
    operationWatcher = new OperationWatcher({
      path: paths.operation,
      pollIntervalMs: 2_000,
      onOperation: async operation => {
        await state.setOperation(operation);
        await updateConsumer?.handle(operation);
      },
      onError: error => logger.error("watch app update operation", error),
    });
    operationWatcher.start();
  }

  await runBootstrap();
}

function registerProtocol(): void {
  const scheme = desktopProtocolName(app.isPackaged);
  if (process.defaultApp && process.argv[1]) {
    app.setAsDefaultProtocolClient(scheme, process.execPath, [resolve(process.argv[1])]);
  } else {
    app.setAsDefaultProtocolClient(scheme);
  }
}

registerProtocol();
if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.on("second-instance", (_event, argv) => {
    const link = lastDeepLink(argv);
    if (link) links.push(link);
    product?.focus();
    boot?.show();
  });
  const launchLink = lastDeepLink(process.argv);
  if (launchLink) links.push(launchLink);
  void app
    .whenReady()
    .then(async () => {
      applyDefaultDenyPermissions(session.defaultSession);
      await start();
    })
    .catch(error => {
      logger.error("start desktop shell", error);
      app.quit();
    });
  app.on("window-all-closed", () => void quitCleanly());
  app.on("before-quit", event => {
    if (cleanupComplete) return;
    event.preventDefault();
    void quitCleanly();
  });
}
