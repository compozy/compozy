import { homedir } from "node:os";
import { registerLinuxDesktop } from "./deep-links/linux-registration";

import { randomUUID } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { performance } from "node:perf_hooks";
import { join, resolve } from "node:path";

import {
  app,
  globalShortcut,
  ipcMain,
  nativeTheme,
  session,
  shell,
  systemPreferences,
} from "electron";

import { BootstrapRunner } from "./bootstrap/bootstrap-runner";
import { createDesktopCommandLineTool } from "./cli/electron-command-line-tool";
import type { DesktopBuildChannel } from "./release/build-channel";
import { verifyRuntimeBundle } from "./bootstrap/bundle-integrity";
import { waitForE2EProductReady } from "./bootstrap/e2e-product-ready";
import type { RuntimeHealthMonitor } from "./bootstrap/runtime-health-monitor";
import { monitorProductRuntime } from "./bootstrap/runtime-reconnect";
import { bootstrapSnapshot } from "./boot/bootstrap-state";
import { createBootControl } from "./boot/boot-control";
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
import { electronDirectoryDialog } from "./product/directory-dialog";
import { createElectronFileOpener } from "./product/electron-file-opener";
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
import { WindowTheme } from "./window/window-theme";
import type { WindowPresentation } from "./window/window-presentation";
import { applyDefaultDenyPermissions } from "./window/security";
import { configureDiskCache, trimOversizedDiskCache } from "./window/cache-policy";
import { installApplicationMenu } from "./window/application-menu";

declare const __COMPOZY_DESKTOP_E2E_BUILD__: boolean;
declare const __COMPOZY_RELEASE_CHANNEL__: DesktopBuildChannel;

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
let installerOwnsQuit = false;
const bootstrapAbort = new AbortController();
let bootstrapFlow: Promise<void> | null = null;
const commandLineTool = createDesktopCommandLineTool(paths, logger, __COMPOZY_DESKTOP_E2E_BUILD__);

if (app.isPackaged && !__COMPOZY_DESKTOP_E2E_BUILD__) {
  for (const name of ["remote-debugging-port", "remote-debugging-pipe", "inspect", "inspect-brk"]) {
    app.commandLine.removeSwitch(name);
  }
}

app.setPath("userData", join(paths.home, "electron"));
configureDiskCache(app.commandLine);
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

/** Cancels and drains bootstrap before releasing shell resources for quit or installer handoff. */
async function cleanup(): Promise<void> {
  if (cleanupPromise) return await cleanupPromise;
  cleanupPromise = (async () => {
    bootstrapAbort.abort();
    const errors: Error[] = [];
    const attempt = async (name: string, step: () => void | Promise<void>): Promise<boolean> => {
      logger.info("desktop shutdown step started", { step: name });
      try {
        await step();
        logger.info("desktop shutdown step completed", { step: name });
        return true;
      } catch (error) {
        logger.error("desktop shutdown step failed", error, { step: name });
        errors.push(error instanceof Error ? error : new Error(String(error)));
        return false;
      }
    };
    try {
      await attempt("bootstrap", async () => {
        await bootstrapFlow;
      });
      await attempt("window state", async () => await product?.flushState());
      await attempt("product bridge", () => productBridge?.unregister());
      await attempt("operation watcher", () => operationWatcher?.stop());
      await attempt("update consumer", () => updateConsumer?.stop());
      await attempt("runtime monitor", () => runtimeMonitor?.stop());
      await attempt("control server", async () => {
        if (controlServer) await controlServer.close();
      });
      await attempt("control token", async () => await deleteControlToken(paths.appToken));
      // Shell resources are drained; a stuck renderer must not hold the final quit handshake.
      const productClosed = await attempt("product window", async () => await product?.close());
      if (productClosed) {
        await attempt("clean marker", async () => await publisher?.markCleanShutdown());
      }
      await attempt("logs", async () => await logger.flush());
      await logger.flush();
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
  if (!installerOwnsQuit) app.quit();
}

/** Initializes the shell and permits verified staged updates before runtime readiness. */
async function start(): Promise<void> {
  await mkdir(paths.home, { recursive: true, mode: 0o700 });
  // Before any window: native chrome, backgrounds and every renderer's
  // prefers-color-scheme follow the renderer's last reported theme.
  const windowTheme = new WindowTheme({
    nativeTheme,
    path: join(paths.home, "desktop-theme.json"),
  });
  await windowTheme.load();
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
  productBridge = new ProductBridgeController({
    ipcMain,
    shortcuts: shortcutPolicy,
    theme: windowTheme,
    directories: electronDirectoryDialog,
    files: createElectronFileOpener({
      origin: () => product?.current()?.origin ?? null,
      log: (message, fields) => logger.info(message, fields),
    }),
    productTarget: () => product?.current() ?? null,
  });
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
  /** Shows the boot window, reopening it (it presents itself on load) after the product closed it. */
  function showBootWindow(): void {
    if (boot && !boot.isDestroyed()) return boot.show();
    boot = new BootWindow({
      pagePath: resources.page,
      preloadPath: resources.bootPreload,
      presentation: windowPresentation,
      theme: windowTheme,
      snapshot: statePublisher.snapshot(),
      onError: error => logger.error("load boot window", error),
    });
  }
  showBootWindow();
  installApplicationMenu(() => product, commandLineTool?.install);
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
  const bootControl = createBootControl({
    openLogs: async () => await shell.openPath(paths.logs),
    openConfig: async () => await shell.openPath(paths.config),
    openExternal: async url => await shell.openExternal(url),
    targetVersion: () => statePublisher.snapshot().to_version,
    quit: () => void quitCleanly(),
    forward: async (method, params) => await controlHandler(method, params),
  });
  ipcMain.handle("boot:control", async (_event, method: unknown, params: unknown) =>
    bootControl(method, params)
  );

  const runner = new BootstrapRunner({
    bundlePath: resources.bundle,
    manifestPath: resources.manifest,
    logPath: paths.bootstrapLog,
    minimumRuntime: MINIMUM_RUNTIME,
    appVersion: app.getVersion(),
    channel: __COMPOZY_RELEASE_CHANNEL__,
    bootId,
  });
  /** Serializes bootstrap attempts and prevents new work after shutdown begins. */
  async function runBootstrap(): Promise<void> {
    if (bootstrapAbort.signal.aborted) return;
    if (bootstrapFlow) return await bootstrapFlow;
    bootstrapFlow = runBootstrapAttempt();
    try {
      await bootstrapFlow;
    } finally {
      bootstrapFlow = null;
    }
  }

  /** Verifies the bundle, observes runtime boot, and publishes a window only while the shell is active. */
  async function runBootstrapAttempt(): Promise<void> {
    try {
      await verifyRuntimeBundle(resources.bundle, resources.manifest);
      if (bootstrapAbort.signal.aborted) return;
      startUpdateConsumer(statePublisher);
      const runtime = await runner.run(async event => {
        if (bootstrapAbort.signal.aborted) return;
        await statePublisher.publish(
          bootstrapSnapshot(event, paths.bootstrapLog, statePublisher.snapshot())
        );
      }, bootstrapAbort.signal);
      if (bootstrapAbort.signal.aborted) return;
      applyDefaultDenyPermissions(session.defaultSession, runtime.origin);
      await statePublisher.setRuntime(runtime.version, runtime.owned);
      if (bootstrapAbort.signal.aborted) return;
      runtimeMonitor?.stop();
      product = new ProductWindow({
        origin: runtime.origin,
        windowStatePath: paths.windowState,
        preloadPath: resources.productPreload,
        presentation: windowPresentation,
        links,
        theme: windowTheme,
        onReady: async () => {
          if (bootstrapAbort.signal.aborted) return;
          await statePublisher.publish({
            state: "product",
            origin: runtime.origin,
            owned: runtime.owned,
          });
          if (bootstrapAbort.signal.aborted) return;
          runtimeMonitor = monitorProductRuntime(runtime, {
            stopped: () => bootstrapAbort.signal.aborted,
            publish: async snapshot => await statePublisher.publish(snapshot),
            reloadProduct: async () => await product?.reload(),
            productLoaded: () => product?.isLoaded() ?? false,
            home: paths.home,
            showBoot: showBootWindow,
            renderBoot: snapshot => boot?.render(snapshot),
            closeProduct: async () => {
              const current = product;
              product = null;
              const route = current?.route();
              try {
                await current?.discard();
              } finally {
                if (route) links.push(productDeepLink(route));
              }
            },
            resolveRuntime: runBootstrap,
            logger,
            logPath: paths.desktopLog,
          });
          runtimeMonitor.start();
          boot?.close();
        },
        onLoadFailure: async error => {
          if (bootstrapAbort.signal.aborted) return;
          await statePublisher.publish({
            state: "error",
            error: {
              code: "load_deadline_exceeded",
              safe_message: publicSafeText(error.message, "The product window did not load."),
              log_path: paths.desktopLog,
            },
          });
          showBootWindow();
        },
        onError: error => logger.error("product window", error),
      });
      await waitForE2EProductReady(bootstrapAbort.signal);
      if (bootstrapAbort.signal.aborted) return;
      await product.create();
      commandLineTool?.firstRun();
    } catch (error) {
      if (bootstrapAbort.signal.aborted) return;
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
      showBootWindow();
    }
  }

  /** Starts one staged-update watcher after packaged runtime integrity has been established. */
  function startUpdateConsumer(state: AppStatePublisher): void {
    if (operationWatcher) return;
    updateConsumer = new AppUpdateConsumer({
      currentVersion: app.getVersion(),
      transitions: new UpdateTransitionClient(resources.bundle),
      installer: new ElectronUpdateInstaller({
        prepareForRestart: async signal => {
          signal.throwIfAborted();
          installerOwnsQuit = true;
          try {
            await cleanup();
            signal.throwIfAborted();
            app.releaseSingleInstanceLock();
          } catch (error) {
            installerOwnsQuit = false;
            throw error;
          }
        },
      }),
      onError: error => {
        logger.error("consume app update operation", error);
        if (cleanupPromise) {
          installerOwnsQuit = false;
          void quitCleanly();
        }
      },
    });
    operationWatcher = new OperationWatcher({
      path: paths.operation,
      pollIntervalMs: 2_000,
      onOperation: async operation => {
        await state.setOperation(operation);
        return (await updateConsumer?.handle(operation)) ?? false;
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
if (process.platform === "win32") app.setAppUserModelId("com.compozy.os");

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
      void trimOversizedDiskCache(session.defaultSession).catch(error => {
        logger.error("trim desktop HTTP cache", error);
      });
      if (process.platform === "linux" && app.isPackaged) {
        try {
          await registerLinuxDesktop(
            process.env.APPIMAGE || process.execPath,
            app.getVersion(),
            process.env.XDG_DATA_HOME || join(homedir(), ".local", "share")
          );
        } catch (error) {
          logger.error("register Linux desktop URI handler", error);
        }
      }
      await start();
    })
    .catch(error => {
      logger.error("start desktop shell", error);
      app.quit();
    });
  app.on("window-all-closed", () => {
    if (!installerOwnsQuit) void quitCleanly();
  });
  app.on("before-quit", event => {
    if (cleanupComplete) return;
    event.preventDefault();
    if (!installerOwnsQuit) void quitCleanly();
  });
}
