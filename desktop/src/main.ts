import { mkdir } from "node:fs/promises";
import { join, resolve } from "node:path";

import { app, ipcMain, session, shell } from "electron";

import { BootstrapRunner } from "./bootstrap/bootstrap-runner";
import { bootstrapSnapshot } from "./boot/bootstrap-state";
import { createControlHandler } from "./control/control-handler";
import { ControlMethodError } from "./control/control-contract";
import { ControlServer } from "./control/control-server";
import { deleteControlToken, rotateControlToken } from "./control/control-token";
import { parseDeepLink, DeepLinkQueue, lastDeepLink } from "./deep-links/deep-link";
import { exportDiagnostics } from "./diagnostics/export-diagnostics";
import { MINIMUM_RUNTIME, RELEASE_CHANNEL } from "./generated/build-config";
import { resolveDesktopPaths } from "./home";
import { DesktopLogger } from "./logging/desktop-logger";
import { AppStatePublisher } from "./state/app-state";
import { publicSafeText } from "./state/public-safe-text";
import { AppUpdateConsumer } from "./update/app-update-consumer";
import { ElectronUpdateInstaller } from "./update/electron-installer";
import { OperationWatcher } from "./update/operation-watcher";
import { UpdateTransitionClient } from "./update/transition-client";
import { BootWindow } from "./window/boot-window";
import { ProductWindow } from "./window/product-window";
import { applyDefaultDenyPermissions } from "./window/security";
import { installApplicationMenu } from "./window/application-menu";

const paths = resolveDesktopPaths();
const links = new DeepLinkQueue();
const logger = new DesktopLogger(paths.desktopLog);
let product: ProductWindow | null = null;
let boot: BootWindow | null = null;
let controlServer: ControlServer | null = null;
let operationWatcher: OperationWatcher | null = null;
let updateConsumer: AppUpdateConsumer | null = null;
let cleanupPromise: Promise<void> | null = null;

if (app.isPackaged) {
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

function resourcePaths(): { bundle: string; manifest: string; page: string; preload: string } {
  const packagedRoot = process.resourcesPath;
  const bundleRoot = process.env.COMPOZY_DESKTOP_BUNDLE_ROOT?.trim()
    ? resolve(process.env.COMPOZY_DESKTOP_BUNDLE_ROOT)
    : join(packagedRoot, "runtime");
  const runtimeName = process.platform === "win32" ? "compozy.exe" : "compozy";
  return {
    bundle: join(bundleRoot, runtimeName),
    manifest: join(bundleRoot, "runtime-manifest.json"),
    page: app.isPackaged
      ? join(packagedRoot, "pages", "boot.html")
      : join(__dirname, "pages", "boot.html"),
    preload: join(__dirname, "boot-preload.cjs"),
  };
}

async function cleanup(): Promise<void> {
  if (cleanupPromise) return await cleanupPromise;
  cleanupPromise = (async () => {
    operationWatcher?.stop();
    updateConsumer?.stop();
    if (controlServer) await controlServer.close();
    await deleteControlToken(paths.appToken);
    await logger.flush();
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
  const publisher = new AppStatePublisher({
    path: paths.appRecord,
    appVersion: app.getVersion(),
    channel: RELEASE_CHANNEL,
    startedAt: new Date(),
    onPublished: snapshot => boot?.render(snapshot),
  });
  boot = new BootWindow({
    pagePath: resources.page,
    preloadPath: resources.preload,
    onError: error => logger.error("load boot window", error),
  });
  installApplicationMenu(() => product);
  await publisher.publish({ state: "resolving" });

  const controlHandler = createControlHandler(publisher, {
    navigate: async path => {
      const target = parseDeepLink(`compozyos://open${path}`);
      if (target.kind !== "product") {
        throw new ControlMethodError("invalid_target_path", "The product path is invalid.");
      }
      links.push(`compozyos://open${target.path}`);
      product?.focus();
      boot?.show();
      return target.path;
    },
    retry: async () => await runBootstrap(),
    exportDiagnostics: async () => await exportDiagnostics(paths.runtime),
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
    if (
      method === "retry" ||
      method === "diagnose" ||
      method === "copy_diagnostics" ||
      method === "export_diagnostics"
    ) {
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
  });
  async function runBootstrap(): Promise<void> {
    try {
      const runtime = await runner.run(async event => {
        await publisher.publish(bootstrapSnapshot(event, paths.bootstrapLog));
      });
      await publisher.setRuntime(runtime.version, true);
      product = new ProductWindow({
        origin: runtime.origin,
        windowStatePath: paths.windowState,
        links,
        onReady: async () => {
          await publisher.publish({ state: "product", origin: runtime.origin, owned: true });
          boot?.close();
        },
        onLoadFailure: async error => {
          await publisher.publish({
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
      await product.create();
      startUpdateConsumer(publisher);
    } catch (error) {
      logger.error("bootstrap runtime", error);
      const terminal = publisher.snapshot();
      if (terminal.state !== "error" && terminal.state !== "skew") {
        await publisher.publish({
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
      transitions: new UpdateTransitionClient(paths.runtime),
      installer: new ElectronUpdateInstaller(),
      onError: error => logger.error("consume app update operation", error),
    });
    operationWatcher = new OperationWatcher(
      paths.operation,
      2_000,
      async operation => {
        await state.setOperation(operation);
        await updateConsumer?.handle(operation);
      },
      error => logger.error("watch app update operation", error)
    );
    operationWatcher.start();
  }

  await runBootstrap();
}

function registerProtocol(): void {
  const scheme = app.isPackaged ? "compozyos" : "compozyos-dev";
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
  app.on("before-quit", () => {
    if (!cleanupPromise)
      void cleanup().catch(error => logger.error("clean desktop shutdown", error));
  });
}
